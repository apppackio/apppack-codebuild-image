package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/rs/zerolog"
)

// The three registry values below used to come from
// github.com/docker/docker/registry. Moby split the engine into its own
// module and that package now only exists under github.com/moby/moby/v2,
// which is still a beta release. They are a pair of constants and a four-line
// string helper, so they are inlined here rather than taking a beta
// dependency on the whole engine for them. Copied from
// github.com/docker/docker@v28.5.2+incompatible/registry (config.go, auth.go).
const (
	// defaultNamespace is the namespace Docker Hub images live in when the
	// user gives no registry at all.
	defaultNamespace = "docker.io"
	// indexServer is the auth endpoint for Docker Hub.
	indexServer = "https://index.docker.io/v1/"
)

// convertToHostname strips any scheme and path from a registry URL, leaving
// the bare host that credentials are stored against.
func convertToHostname(url string) string {
	stripped := url
	if after, ok := strings.CutPrefix(stripped, "http://"); ok {
		stripped = after
	} else if after, ok := strings.CutPrefix(stripped, "https://"); ok {
		stripped = after
	}
	stripped, _, _ = strings.Cut(stripped, "/")
	return stripped
}

type BuildConfig struct {
	Image     string // {repo}:{commit}
	LatestTag string // {repo}:latest
	BuildTag  string // {repo}:build-{buildnumber}
	CacheDir  string
	LogFile   *os.File
	Env       map[string]string
}

func NewBuildConfig(image, buildNumber string, env map[string]string, logFile *os.File, cacheDir string) *BuildConfig {
	return &BuildConfig{
		Image:     image,
		LatestTag: "latest",
		BuildTag:  "build-" + buildNumber,
		CacheDir:  cacheDir,
		LogFile:   logFile,
		Env:       env,
	}
}

type ContainersI interface {
	Close() error
	CreateNetwork(context.Context, string) error
	PullImage(context.Context, string) error
	PushImage(context.Context, string) error
	BuildImage(context.Context, string, *BuildConfig) error
	CreateContainer(context.Context, string, *container.Config) (*string, error)
	DeleteContainer(context.Context, string) error
	RunContainer(context.Context, string, string, []string, *container.Config) error
	GetContainerFile(context.Context, string, string) (io.ReadCloser, error)
	WaitForExit(context.Context, string) (int, error)
	AttachLogs(context.Context, string, io.Writer, io.Writer) error
}

type Containers struct {
	cli    *client.Client
	logger *zerolog.Logger
}

func New(logger *zerolog.Logger) (*Containers, error) {
	// API-version negotiation is on by default in this client, so the old
	// WithAPIVersionNegotiation() option is gone along with NewClientWithOpts.
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Containers{
		cli:    cli,
		logger: logger,
	}, nil
}

func (c *Containers) Log() *zerolog.Logger {
	return c.logger
}

func (c *Containers) Close() error {
	return c.cli.Close()
}

// borrowed from
// https://github.com/google/go-containerregistry/blob/b3c23b4c3f283a36b5c68565ebef4b266f1fb29f/cmd/crane/cmd/auth.go#L171
func Login(serverAddress string, user string, password string) error {
	if user == "" && password == "" {
		return errors.New("username and password required")
	}

	if serverAddress == "" || serverAddress == defaultNamespace {
		serverAddress = indexServer
	}

	isDefaultRegistry := serverAddress == indexServer
	if !isDefaultRegistry {
		serverAddress = convertToHostname(serverAddress)
	}

	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return err
	}
	creds := cf.GetCredentialsStore(serverAddress)
	if err := creds.Store(types.AuthConfig{
		ServerAddress: serverAddress,
		Username:      user,
		Password:      password,
	}); err != nil {
		return err
	}

	return cf.Save()
}

func (c *Containers) CreateNetwork(ctx context.Context, id string) error {
	c.Log().Debug().Str("network", id).Msg("creating docker network")
	_, err := c.cli.NetworkCreate(ctx, id, client.NetworkCreateOptions{})
	return err
}

func (c *Containers) PullImage(ctx context.Context, imageName string) error {
	c.Log().Debug().Str("image", imageName).Msg("pulling image")
	cmd := exec.CommandContext(ctx, "docker", "pull", imageName) //nolint:gosec // G204: the image name is the one the build was told to pull.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Containers) PushImage(ctx context.Context, imageName string) error {
	c.Log().Debug().Str("image", imageName).Msg("pushing image")
	cmd := exec.CommandContext(ctx, "docker", "push", imageName) //nolint:gosec // G204: the image name is the one the build was told to push.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Containers) CreateContainer(ctx context.Context, name string, config *container.Config) (*string, error) {
	c.Log().Debug().Str("image", config.Image).Str("name", name).Msg("creating container")
	resp, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           config,
		NetworkingConfig: &network.NetworkingConfig{},
		Name:             name,
	})
	if err != nil {
		return nil, err
	}
	return &resp.ID, nil
}

// RunContainer creates and starts a container connected to networkID. The
// container is created with the (unique) name so it never collides on a
// reused Docker daemon. Any aliases are registered as network-scoped
// hostnames so other containers can reach it by a friendly name (e.g. "db").
func (c *Containers) RunContainer(ctx context.Context, name string, networkID string, aliases []string, config *container.Config) error {
	c.Log().Debug().Str("image", config.Image).Str("container", name).Strs("aliases", aliases).Msg("starting container")
	containerID, err := c.CreateContainer(ctx, name, config)
	if err != nil {
		return err
	}
	var endpoint *network.EndpointSettings
	if len(aliases) > 0 {
		endpoint = &network.EndpointSettings{Aliases: aliases}
	}
	_, err = c.cli.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{
		Container:      *containerID,
		EndpointConfig: endpoint,
	})
	if err != nil {
		return err
	}
	_, err = c.cli.ContainerStart(ctx, *containerID, client.ContainerStartOptions{})
	return err
}

func (c *Containers) GetContainerFile(ctx context.Context, containerID string, src string) (io.ReadCloser, error) {
	c.Log().Debug().Str("file", src).Str("container", containerID).Msg("copying file from container")
	res, err := c.cli.CopyFromContainer(ctx, containerID, client.CopyFromContainerOptions{SourcePath: src})
	if err != nil {
		return nil, err
	}
	return res.Content, nil
}

// WaitContainer waits for a container to exit and returns the exit code
func (c *Containers) WaitForExit(ctx context.Context, containerID string) (int, error) {
	c.Log().Debug().Str("container", containerID).Msg("waiting for container to exit")
	res := c.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case err := <-res.Error:
		return -1, err
	case status := <-res.Result:
		return int(status.StatusCode), nil
	}
}

// AttachLogs attaches to the logs of a container and writes them to stdout
func (c *Containers) AttachLogs(ctx context.Context, containerID string, stdout, stderr io.Writer) error {
	c.Log().Debug().Str("container", containerID).Msg("attaching to logs of container")
	// stream stdout and stderr from the container to the host
	// use stdcopy to separate the streams
	reader, err := c.cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	_, err = stdcopy.StdCopy(stdout, stderr, reader)
	return err
}

func (c *Containers) DeleteContainer(ctx context.Context, containerID string) error {
	c.Log().Debug().Str("container", containerID).Msg("deleting container")
	_, err := c.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true})
	return err
}

func (c *Containers) BuildImage(ctx context.Context, dockerfile string, config *BuildConfig) error {
	c.Log().Debug().Str("image", config.Image).Msg("building Docker image")
	dockerArgs := []string{
		"buildx",
		"build",
		"--tag", config.Image,
		"--progress", "plain",
		"--cache-to", "type=local,dest=" + config.CacheDir,
		"--cache-from", "type=local,src=" + config.CacheDir,
		"--file", dockerfile,
		"--load",
	}
	for k, v := range config.Env {
		dockerArgs = append(dockerArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	dockerArgs = append(dockerArgs, ".")
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...) //nolint:gosec // G204: the args are the app's own build config; running docker over it is what a build is.
	out := io.MultiWriter(os.Stdout, config.LogFile)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
