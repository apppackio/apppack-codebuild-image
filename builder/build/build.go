package build

import (
	"fmt"
	"strings"

	"github.com/buildpacks/pack/pkg/client"
	"github.com/buildpacks/pack/pkg/image"
	"github.com/docker/docker/api/types/container"
)

func stripParamPrefix(params map[string]string, prefix string, final *map[string]string) {
	for k, v := range params {
		// strip prefix from k and add to final
		(*final)[strings.TrimPrefix(k, prefix)] = v
	}
}

func (b *Build) LoadBuildEnv() (map[string]string, error) {
	paths := b.ConfigParameterPaths()
	env := map[string]string{}
	params, err := b.aws.GetParametersByPath(paths[0])
	stripParamPrefix(params, paths[0], &env)
	if err != nil {
		return nil, err
	}
	if len(paths) > 1 {
		// overlay vars from additional paths (for review apps)
		for _, path := range paths[1:] {
			params, err := b.aws.GetParametersByPath(path)
			if err != nil {
				return nil, err
			}
			stripParamPrefix(params, path, &env)
		}
	}
	envOverride, err := b.state.ReadEnvFile()
	if err != nil {
		b.Log.Debugf("Failed to read env file %s", err)
	}
	for k, v := range *envOverride {
		env[k] = v
	}
	return env, nil
}

func (b *Build) RunBuild() error {
	skipBuild, _ := b.state.ShouldSkipBuild(b.CodebuildBuildId)
	if skipBuild {
		b.Log.Info("Skipping build")
		return nil
	}
	pack, err := client.NewClient(client.WithLogger(b.Log))
	if err != nil {
		return err
	}
	appEnv, err := b.LoadBuildEnv()
	if err != nil {
		return err
	}
	imageName, err := b.ImageName()
	if err != nil {
		return err
	}
	PrintStartMarker("build")
	defer PrintEndMarker("build")
	err = pack.Build(b.Context, client.BuildOptions{
		AppPath:    ".",
		Builder:    b.AppJSON.GetBuilders()[0],
		Buildpacks: b.AppJSON.GetBuildpacks(),
		Env:        appEnv,
		Image:      fmt.Sprintf("%s:latest", b.ECRRepo),
		CacheImage: fmt.Sprintf("%s:cache", b.ECRRepo),
		AdditionalTags: []string{
			fmt.Sprintf("%s:build-%s", b.ECRRepo, b.CodebuildBuildNumber),
			imageName,
		},
		PreviousImage: fmt.Sprintf("%s:latest", b.ECRRepo),
		Publish:       true,
		PullPolicy:    image.PullIfNotPresent,
		// TrustBuilder:  func(string) bool { return true },
	})
	if err != nil {
		return err
	}
	defer b.containers.Close()
	if err = b.containers.PullImage(imageName, b.Log); err != nil {
		return err
	}
	containerID, err := b.containers.CreateContainer(b.Appname, &container.Config{Image: imageName})
	if err != nil {
		return err
	}
	reader, err := b.containers.GetContainerFile(*containerID, "/layers/config/metadata.toml")
	if err != nil {
		return err
	}
	return b.state.WriteMetadataToml(reader)
}
