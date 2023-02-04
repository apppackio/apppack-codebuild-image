package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	apiTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/registry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

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
		BuildTag:  fmt.Sprintf("build-%s", buildNumber),
		CacheDir:  cacheDir,
		LogFile:   logFile,
		Env:       env,
	}
}

type ContainersI interface {
	Close() error
	CreateNetwork(string) error
	PullImage(string) error
	PushImage(string) error
	BuildImage(string, *BuildConfig) error
	CreateContainer(string, *container.Config) (*string, error)
	DeleteContainer(string) error
	RunContainer(string, string, *container.Config) error
	GetContainerFile(string, string) (io.ReadCloser, error)
	WaitForExit(string) (int, error)
	AttachLogs(string, io.Writer, io.Writer) error
}

type Containers struct {
	ctx context.Context
	cli *client.Client
}

func New(ctx context.Context) (*Containers, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Containers{
		ctx: ctx,
		cli: cli,
	}, nil
}

func (c *Containers) Log() *zerolog.Logger {
	return log.Ctx(c.ctx)
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

	if serverAddress == "" || serverAddress == registry.DefaultNamespace {
		serverAddress = registry.IndexServer
	}

	isDefaultRegistry := serverAddress == registry.IndexServer
	if !isDefaultRegistry {
		serverAddress = registry.ConvertToHostname(serverAddress)
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

func (c *Containers) CreateNetwork(id string) error {
	c.Log().Debug().Str("network", id).Msg("creating docker network")
	_, err := c.cli.NetworkCreate(c.ctx, id, apiTypes.NetworkCreate{})
	return err
}

func (c *Containers) PullImage(imageName string) error {
	c.Log().Debug().Str("image", imageName).Msg("pulling image")
	cmd := exec.Command("docker", "pull", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Containers) PushImage(imageName string) error {
	c.Log().Debug().Str("image", imageName).Msg("pushing image")
	cmd := exec.Command("docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Containers) CreateContainer(name string, config *container.Config) (*string, error) {
	c.Log().Debug().Str("image", config.Image).Str("name", name).Msg("creating container")
	resp, err := c.cli.ContainerCreate(c.ctx, config, nil, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return nil, err
	}
	return &resp.ID, nil
}

func (c *Containers) RunContainer(name string, networkID string, config *container.Config) error {
	c.Log().Debug().Str("image", config.Image).Str("container", name).Msg("starting container")
	containerID, err := c.CreateContainer(name, config)
	if err != nil {
		return err
	}
	err = c.cli.NetworkConnect(c.ctx, networkID, *containerID, nil)
	if err != nil {
		return err
	}
	return c.cli.ContainerStart(c.ctx, *containerID, apiTypes.ContainerStartOptions{})
}

func (c *Containers) GetContainerFile(containerID string, src string) (io.ReadCloser, error) {
	c.Log().Debug().Str("file", src).Str("container", containerID).Msg("copying file from container")
	reader, _, err := c.cli.CopyFromContainer(c.ctx, containerID, src)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// WaitContainer waits for a container to exit and returns the exit code
func (c *Containers) WaitForExit(containerID string) (int, error) {
	c.Log().Debug().Str("container", containerID).Msg("waiting for container to exit")
	statusCh, errCh := c.cli.ContainerWait(c.ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return -1, err
	case status := <-statusCh:
		return int(status.StatusCode), nil
	}
}

// AttachLogs attaches to the logs of a container and writes them to stdout
func (c *Containers) AttachLogs(containerID string, stdout, stderr io.Writer) error {
	c.Log().Debug().Str("container", containerID).Msg("attaching to logs of container")
	// stream stdout and stderr from the container to the host
	// use stdcopy to separate the streams
	reader, err := c.cli.ContainerLogs(c.ctx, containerID, apiTypes.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = stdcopy.StdCopy(stdout, stderr, reader)
	return err

}

func (c *Containers) DeleteContainer(containerID string) error {
	c.Log().Debug().Str("container", containerID).Msg("deleting container")
	return c.cli.ContainerRemove(c.ctx, containerID, apiTypes.ContainerRemoveOptions{Force: true})
}

func (c *Containers) BuildImage(dockerfile string, config *BuildConfig) error {
	c.Log().Debug().Str("image", config.Image).Msg("building Docker image")
	cacheArg := fmt.Sprintf("type=local,src=%s", config.CacheDir)
	cmd := exec.Command(
		"docker", "buildx", "build",
		"--tag", config.Image,
		"--progress", "plain",
		"--cache-to", cacheArg,
		"--cache-from", cacheArg,
		"--file", dockerfile,
		"--load",
		".",
	)
	out := io.MultiWriter(os.Stdout, config.LogFile)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
