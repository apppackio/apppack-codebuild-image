package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

func Login(serverAddress string, user string, password string) error {
	if user == "" && password == "" {
		return errors.New("username and password required")
	}
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.RegistryLogin(ctx, types.AuthConfig{
		Username:      user,
		Password:      password,
		ServerAddress: serverAddress,
	})
	return err
}

func CreateDockerNetwork(id string) error {
	log.Debug().Msg("creating docker network")
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.NetworkCreate(ctx, id, types.NetworkCreate{})
	return err
}

func PullImage(image string) error {
	log.Debug().Msgf("pulling %s", image)
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	out, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()
	return nil
}

func CreateContainer(image string, name string) (*string, error) {
	log.Debug().Msg(fmt.Sprintf("creating container for %s", image))
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	resp, err := cli.ContainerCreate(ctx, &container.Config{Image: image}, nil, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return nil, err
	}
	return &resp.ID, nil
}

func RunContainer(image string, name string, networkID string) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	log.Debug().Msg(fmt.Sprintf("starting container for %s", image))
	containerID, err := CreateContainer(image, name)
	if err != nil {
		return err
	}
	err = cli.NetworkConnect(ctx, networkID, *containerID, nil)
	if err != nil {
		return err
	}
	return cli.ContainerStart(ctx, *containerID, types.ContainerStartOptions{})
}

func CopyContainerFile(containerID string, src string, dest string) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	log.Debug().Msg(fmt.Sprintf("copying file from %s to %s", src, dest))
	reader, _, err := cli.CopyFromContainer(ctx, containerID, src)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return nil
}
