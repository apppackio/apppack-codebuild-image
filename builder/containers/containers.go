package containers

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/buildpacks/pack/pkg/image"
	"github.com/buildpacks/pack/pkg/logging"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	apiTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type ContainersI interface {
	Close() error
	CreateDockerNetwork(string) error
	PullImage(string, logging.Logger) error
	CreateContainer(string, *container.Config) (*string, error)
	DeleteContainer(string) error
	RunContainer(string, string, *container.Config) error
	GetContainerFile(string, string) (io.ReadCloser, error)
	WaitForExit(string) (int, error)
	AttachLogs(string) error
}

type Containers struct {
	context context.Context
	cli     *client.Client
	logger  logging.Logger
}

func New(ctx context.Context, logger logging.Logger) (*Containers, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Containers{
		context: ctx,
		cli:     cli,
		logger:  logger,
	}, nil
}

func (c *Containers) Close() error {
	return c.cli.Close()
}

func Login(serverAddress string, user string, password string) error {
	if user == "" && password == "" {
		return errors.New("username and password required")
	}

	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return err
	}
	creds := cf.GetCredentialsStore(serverAddress)
	if serverAddress == name.DefaultRegistry {
		serverAddress = authn.DefaultAuthKey
	}
	if err := creds.Store(types.AuthConfig{
		ServerAddress: registry.ConvertToHostname(serverAddress),
		Username:      user,
		Password:      password,
	}); err != nil {
		return err
	}

	if err := cf.Save(); err != nil {
		return err
	}
	return nil
}

func (c *Containers) CreateDockerNetwork(id string) error {
	c.logger.Debugf("creating docker network: %s", id)
	_, err := c.cli.NetworkCreate(c.context, id, apiTypes.NetworkCreate{})
	return err
}

func (c *Containers) PullImage(imageName string, logger logging.Logger) error {
	c.logger.Debugf("pulling %s", imageName)
	fetcher := image.NewFetcher(logger, c.cli)
	_, err := fetcher.Fetch(c.context, imageName, image.FetchOptions{Daemon: true})
	return err
}

func (c *Containers) CreateContainer(name string, config *container.Config) (*string, error) {
	c.logger.Debugf("creating container for %s", config.Image)
	resp, err := c.cli.ContainerCreate(c.context, config, nil, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return nil, err
	}
	return &resp.ID, nil
}

func (c *Containers) RunContainer(name string, networkID string, config *container.Config) error {
	c.logger.Debugf("starting container for %s", config.Image)
	containerID, err := c.CreateContainer(name, config)
	if err != nil {
		return err
	}
	err = c.cli.NetworkConnect(c.context, networkID, *containerID, nil)
	if err != nil {
		return err
	}
	return c.cli.ContainerStart(c.context, *containerID, apiTypes.ContainerStartOptions{})
}

func (c *Containers) GetContainerFile(containerID string, src string) (io.ReadCloser, error) {
	c.logger.Debugf("copying file from %s", src)
	reader, _, err := c.cli.CopyFromContainer(c.context, containerID, src)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// WaitContainer waits for a container to exit and returns the exit code
func (c *Containers) WaitForExit(containerID string) (int, error) {
	c.logger.Debugf("waiting for container %s to exit", containerID)
	statusCh, errCh := c.cli.ContainerWait(c.context, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return -1, err
	case status := <-statusCh:
		return int(status.StatusCode), nil
	}
}

// AttachLogs attaches to the logs of a container and writes them to stdout
func (c *Containers) AttachLogs(containerID string) error {
	c.logger.Debugf("attaching to logs of container %s", containerID)
	// stream stdout and stderr from the container to the host
	// use stdcopy to separate the streams
	reader, err := c.cli.ContainerLogs(c.context, containerID, apiTypes.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
	return err

}

func (c *Containers) DeleteContainer(containerID string) error {
	c.logger.Debugf("deleting container %s", containerID)
	return c.cli.ContainerRemove(c.context, containerID, apiTypes.ContainerRemoveOptions{Force: true})
}
