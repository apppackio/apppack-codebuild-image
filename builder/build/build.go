package build

import (
	"fmt"
	"strings"

	"github.com/apppackio/codebuild-image/builder/logs"
	"github.com/buildpacks/pack/pkg/client"
	"github.com/buildpacks/pack/pkg/image"
	"github.com/docker/docker/api/types/container"
)

const DockerHubMirror = "registry.apppackcdn.net"

func stripParamPrefix(params map[string]string, prefix string, final *map[string]string) {
	for k, v := range params {
		// strip prefix from k and add to final
		(*final)[strings.TrimPrefix(k, prefix)] = v
	}
}

func (b *Build) LoadBuildEnv() (map[string]string, error) {
	paths := b.ConfigParameterPaths()
	env := map[string]string{
		"CI": "true",
	}
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
		b.Log().Debug().Err(err).Msg("cannot read env file")
	}
	for k, v := range *envOverride {
		env[k] = v
	}
	return env, nil
}

func (b *Build) RunBuild() error {
	skipBuild, _ := b.state.ShouldSkipBuild(b.CodebuildBuildId)
	if skipBuild {
		b.Log().Info().Msg("skipping build")
		return nil
	}
	logFile, err := b.state.CreateLogFile("build.log")
	if err != nil {
		return err
	}
	defer logFile.Close()
	buildLogs, err := logs.PackLoggerToFileFromZerolog(b.Log(), logFile)
	if err != nil {
		return err
	}
	pack, err := client.NewClient(
		client.WithLogger(buildLogs),
		client.WithRegistryMirrors(map[string]string{"index.docker.io": DockerHubMirror}),
	)
	if err != nil {
		return err
	}
	b.Log().Debug().Msg("loading build environment variables")
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
	err = pack.Build(b.Ctx, client.BuildOptions{
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
	if err = b.containers.PullImage(imageName, logs.WithQuiet()); err != nil {
		return err
	}
	containerID := fmt.Sprintf("%s-%s", b.Appname, strings.ReplaceAll(b.CodebuildBuildId, ":", "-"))
	cid, err := b.containers.CreateContainer(containerID, &container.Config{Image: imageName})
	if err != nil {
		return err
	}
	defer b.containers.DeleteContainer(*cid)
	reader, err := b.containers.GetContainerFile(*cid, "/layers/config/metadata.toml")
	if err != nil {
		return err
	}
	defer reader.Close()
	err = b.state.UnpackTarArchive(reader)
	if err != nil {
		return err
	}
	return b.state.WriteCommitTxt()
}
