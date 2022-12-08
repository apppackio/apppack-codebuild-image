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
	"github.com/docker/docker/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type ContainersI interface {
	Close() error
	CreateDockerNetwork(string) error
	PullImage(string, logging.Logger) error
	CreateContainer(string, string) (*string, error)
	RunContainer(string, string, string) error
	GetContainerFile(string, string) (io.ReadCloser, error)
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
	c.logger.Debug("creating docker network")
	_, err := c.cli.NetworkCreate(c.context, id, apiTypes.NetworkCreate{})
	return err
}

func (c *Containers) PullImage(imageName string, logger logging.Logger) error {
	c.logger.Debugf("pulling %s", imageName)
	fetcher := image.NewFetcher(logger, c.cli)
	_, err := fetcher.Fetch(c.context, imageName, image.FetchOptions{Daemon: true})
	return err
}

func (c *Containers) CreateContainer(image string, name string) (*string, error) {
	c.logger.Debugf("creating container for %s", image)
	resp, err := c.cli.ContainerCreate(c.context, &container.Config{Image: image}, nil, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return nil, err
	}
	return &resp.ID, nil
}

func (c *Containers) RunContainer(image string, name string, networkID string) error {
	c.logger.Debugf("starting container for %s", image)
	containerID, err := c.CreateContainer(image, name)
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
