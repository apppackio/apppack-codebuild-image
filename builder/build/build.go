package build

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/docker/docker/api/types/container"
	"github.com/rs/zerolog"
)

const DockerHubMirror = "registry.apppackcdn.net"
const CacheDirectory = "/tmp/apppack-cache"

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
	} else {
		for k, v := range *envOverride {
			env[k] = v
		}
	}
	return env, nil
}

func (b *Build) BuildpackBuilders() []string {
	if b.AppPackToml.Build.Builder != "" {
		return []string{b.AppPackToml.Build.Builder}
	}
	return b.AppJSON.GetBuilders()
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

	b.Log().Debug().Msg("loading build environment variables")
	appEnv, err := b.LoadBuildEnv()
	if err != nil {
		return err
	}
	imageName, err := b.ImageName()
	if err != nil {
		return err
	}
	buildConfig := containers.NewBuildConfig(imageName, b.CodebuildBuildNumber, appEnv, logFile)
	PrintStartMarker("build")
	defer PrintEndMarker("build")
	if b.AppPackToml.UseDockerfile() {
		err = b.buildWithDocker(buildConfig)
	} else {
		err = b.buildWithPack(buildConfig)
	}
	if err != nil {
		return err
	}

	return b.state.WriteCommitTxt()
}

func (b *Build) buildWithDocker(config *containers.BuildConfig) error {
	defer b.containers.Close()
	defer config.LogFile.Close()

	if err := b.containers.BuildImage(b.AppPackToml.Build.Dockerfile, config); err != nil {
		return err
	}
	metadataToml := b.AppPackToml.ToMetadataToml()
	return metadataToml.Write(b.Ctx)
}

func (b *Build) buildWithPack(config *containers.BuildConfig) error {
	b.Log().Debug().Msg("pack config registry-mirrors")
	cmd := exec.Command("pack", "config", "registry-mirrors", "add", "index.docker.io", "--mirror", DockerHubMirror)
	if err := cmd.Run(); err != nil {
		return err
	}
	builder := b.BuildpackBuilders()[0]
	buildpacks := strings.Join(b.AppJSON.GetBuildpacks(), ",")
	packArgs := []string{
		"build",
		"--builder", builder,
		"--buildpack", buildpacks,
		"--cache", fmt.Sprintf("type=build;format=bind;source=%s", CacheDirectory),
		"--tag", config.BuildImage,
		"--tag", config.Image,
		"--pull-policy", "if-not-present",
	}
	for k, v := range config.Env {
		packArgs = append(packArgs, "--env", fmt.Sprintf("%s=%s", k, v))
	}
	if b.Log().GetLevel() <= zerolog.DebugLevel {
		packArgs = append(packArgs, "--verbose", "--timestamps")
	}
	packArgs = append(packArgs, config.LatestImage)
	b.Log().Debug().Str("builder", builder).Str("buildpacks", buildpacks).Msg("building image")
	cmd = exec.Command("pack", packArgs...)
	out := io.MultiWriter(os.Stdout, config.LogFile)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return err
	}
	var wgErrors []error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Log().Debug().Msg("syncing build cache to s3")
		wgErrors = append(wgErrors, b.aws.SyncToS3(CacheDirectory, b.ArtifactBucket, "cache"))
	}()
	err := b.containers.PushImage(config.BuildImage)
	if err != nil {
		return err
	}
	// once the first image is pushed, we can push the rest in parallel
	// so they can share the same layers
	for _, img := range []string{config.BuildImage, config.LatestImage} {
		wg.Add(1)
		go func(img string) {
			defer wg.Done()
			wgErrors = append(wgErrors, b.containers.PushImage(img, containers.WithQuiet()))
		}(img)
	}
	defer b.containers.Close()
	containerID := fmt.Sprintf("%s-%s", b.Appname, strings.ReplaceAll(b.CodebuildBuildId, ":", "-"))
	cid, err := b.containers.CreateContainer(containerID, &container.Config{Image: config.Image})
	if err != nil {
		return err
	}
	defer b.containers.DeleteContainer(*cid)
	reader, err := b.containers.GetContainerFile(*cid, "/layers/config/metadata.toml")
	if err != nil {
		return err
	}
	defer reader.Close()
	if err := b.state.UnpackTarArchive(reader); err != nil {
		return err
	}
	wg.Wait()
	for _, err := range wgErrors {
		if err != nil {
			return err
		}
	}
	return nil
}
