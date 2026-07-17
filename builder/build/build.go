package build

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/apppackio/codebuild-image/builder/filesystem"
	"github.com/docker/docker/api/types/container"
	"github.com/google/go-containerregistry/pkg/crane"
	cp "github.com/otiai10/copy"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	DockerHubMirror       = "registry.apppackcdn.net"
	CacheDirectory        = "/tmp/apppack-cache"
	DefaultMaxCacheSizeGB = 7
	MaxCacheSizeEnvVar    = "APPPACK_MAX_CACHE_SIZE_GB"
)

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
	// pass ALLOW_EOL_SHIMMED_BUILDER to pack if it is in the environment
	// this facilitates testing of the eol shimmed builder
	if val, ok := os.LookupEnv("ALLOW_EOL_SHIMMED_BUILDER"); ok {
		env["ALLOW_EOL_SHIMMED_BUILDER"] = val
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
	logFileName := "build.log"
	logFile, err := os.CreateTemp("", logFileName)
	if err != nil {
		return err
	}
	defer b.state.EndLogging(logFile, logFileName)

	b.Log().Debug().Msg("loading build environment variables")
	appEnv, err := b.LoadBuildEnv()
	if err != nil {
		return err
	}
	imageName, err := b.ImageName()
	if err != nil {
		return err
	}
	buildConfig := containers.NewBuildConfig(imageName, b.CodebuildBuildNumber, appEnv, logFile, CacheDirectory)
	PrintStartMarker("build")
	defer PrintEndMarker("build")
	if b.System() == DockerBuildSystemKeyword {
		err = b.buildWithDocker(buildConfig)
	} else {
		err = b.buildWithPack(buildConfig)
	}
	if err != nil {
		return err
	}
	fmt.Println("===> PUBLISHING")
	var wg sync.WaitGroup
	wg.Add(1)
	var cacheArchiveError error
	go func() {
		defer wg.Done()
		cacheArchiveError = b.archiveCache()
	}()
	if err = b.pushImages(buildConfig); err != nil {
		return err
	}
	wg.Wait()
	if cacheArchiveError != nil {
		return cacheArchiveError
	}
	if err = cp.Copy(logFile.Name(), "build.log"); err != nil {
		return err
	}
	// Copy the apppack.toml file to the default location if it was read from a custom location
	if err = filesystem.CopyAppPackTomlToDefault(); err != nil {
		b.Log().Warn().Err(err).Msg("Failed to copy apppack.toml to default location for artifact archival")
		// Don't fail the build if we can't copy the file, just warn
	}
	return b.state.WriteCommitTxt()
}

func (b *Build) buildWithDocker(config *containers.BuildConfig) error {
	defer b.containers.Close()
	defer config.LogFile.Close()
	dockerfile := b.AppPackToml.Build.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	return b.containers.BuildImage(dockerfile, config)
}

func (b *Build) buildWithPack(config *containers.BuildConfig) error {
	b.Log().Debug().Msg("pack config registry-mirrors")
	builder := b.BuildpackBuilders()[0]
	cmd := exec.Command("pack", "config", "registry-mirrors", "add", "index.docker.io", "--mirror", DockerHubMirror)
	if err := cmd.Run(); err != nil {
		return err
	}
	buildpacks := strings.Join(b.AppJSON.GetBuildpacks(), ",")
	packArgs := []string{
		"build",
		"--builder", builder,
		"--buildpack", buildpacks,
		"--cache", fmt.Sprintf("type=build;format=bind;source=%s", CacheDirectory),
		"--pull-policy", "if-not-present",
	}
	for k, v := range config.Env {
		packArgs = append(packArgs, "--env", fmt.Sprintf("%s=%s", k, v))
	}
	if b.Log().GetLevel() <= zerolog.DebugLevel {
		packArgs = append(packArgs, "--verbose", "--timestamps")
	}
	packArgs = append(packArgs, config.Image)
	b.Log().Debug().Str("builder", builder).Str("buildpacks", buildpacks).Msg("building image")
	cmd = exec.Command("pack", packArgs...)
	out := io.MultiWriter(os.Stdout, config.LogFile)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Println("Extracting buildpack metadata")
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
	b.Log().Debug().Err(err).Msg("converting metadata.toml processes to apppack.toml services")
	metadataToml, err := ParseBuildpackMetadataToml(b.Ctx)
	if err != nil {
		return err
	}
	if b.AppPackToml == nil {
		b.AppPackToml = &AppPackToml{}
	}
	metadataToml.UpdateAppPackToml(b.AppPackToml)
	return b.AppPackToml.Write(b.Ctx)
}

func (b *Build) pushImages(config *containers.BuildConfig) error {
	fmt.Println("Pushing image tag", strings.Split(config.Image, ":")[1])
	err := b.containers.PushImage(config.Image)
	if err != nil {
		return err
	}
	// once the first image is pushed, tag the other images
	for _, tag := range []string{config.BuildTag, config.LatestTag} {
		if err = crane.Tag(config.Image, tag); err != nil {
			return err
		}
	}
	return nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func getMaxCacheSizeGB() int {
	maxGB := DefaultMaxCacheSizeGB
	if val := os.Getenv(MaxCacheSizeEnvVar); val != "" {
		parsed, err := strconv.Atoi(val)
		if err != nil {
			log.Warn().Str("value", val).Int("default_gb", DefaultMaxCacheSizeGB).Msg("invalid " + MaxCacheSizeEnvVar + " value, using default")
		} else if parsed < 0 {
			log.Warn().Int("value", parsed).Int("default_gb", DefaultMaxCacheSizeGB).Msg("negative " + MaxCacheSizeEnvVar + " value, using default")
		} else {
			maxGB = parsed // 0 disables the limit
		}
	}
	return maxGB
}

func (b *Build) archiveCache() error {
	fmt.Println("Archiving build cache to S3 ...")
	maxGB := getMaxCacheSizeGB()
	if maxGB > 0 {
		maxSize := int64(maxGB) * 1024 * 1024 * 1024
		size, err := dirSize(CacheDirectory)
		if err != nil {
			b.Log().Warn().Err(err).Msg("failed to calculate cache directory size")
		} else if size > maxSize {
			b.Log().Warn().Int64("size_bytes", size).Int("max_gb", maxGB).Msg("cache directory exceeds size limit, skipping upload")
			return nil
		}
	}
	quiet := b.Log().GetLevel() > zerolog.DebugLevel
	return b.aws.SyncToS3(CacheDirectory, b.ArtifactBucket, "cache", quiet)
}
