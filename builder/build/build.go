package build

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/apppackio/codebuild-image/builder/awshelpers"
	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/buildpacks/pack/pkg/client"
	"github.com/buildpacks/pack/pkg/image"
	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
)

// GitSha returns the git hash of the current commit
func GitSha() (string, error) {
	cmd, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(cmd)), nil
}

func (b *Build) LoadEnv() (map[string]string, error) {
	paths := b.ConfigParameterPaths()
	params, err := awshelpers.GetParametersByPath(*b.AWSConfig, paths[0])
	if err != nil {
		return nil, err
	}
	if len(paths) == 1 {
		return params, nil
	}
	// overlay vars from additional paths (for review apps)
	for _, path := range paths[1:] {
		p, err := awshelpers.GetParametersByPath(*b.AWSConfig, path)
		if err != nil {
			return nil, err
		}
		for k, v := range p {
			params[k] = v
		}
	}
	return params, nil
}

func (b *Build) RunBuild() error {
	logger := logging.NewLogWithWriters(color.Stdout(), color.Stderr())
	cl, err := client.NewClient(client.WithLogger(logger))
	if err != nil {
		return err
	}
	appJson, err := ParseAppJson()
	if err != nil {
		return err
	}
	gitsha, err := GitSha()
	if err != nil {
		return err
	}
	appEnv, err := b.LoadEnv()
	if err != nil {
		return err
	}
	// color.Disable(true)
	imageName := fmt.Sprintf("%s:%s", b.ECRRepo, gitsha)
	PrintStartMarker("build")
	cl.Build(context.Background(), client.BuildOptions{
		AppPath:    ".",
		Builder:    appJson.GetBuilders()[0],
		Buildpacks: appJson.GetBuildpacks(),
		Env:        appEnv,
		Image:      imageName,
		CacheImage: fmt.Sprintf("%s:cache", b.ECRRepo),
		AdditionalTags: []string{
			fmt.Sprintf("%s:build-%s", b.ECRRepo, b.CodebuildBuildNumber),
			imageName,
		},
		Publish:    true,
		PullPolicy: image.PullIfNotPresent,
	})
	PrintEndMarker("build")
	if err = containers.PullImage(imageName); err != nil {
		return err
	}
	containerID, err := containers.CreateContainer(imageName, b.Appname)
	if err != nil {
		return err
	}
	if err = containers.CopyContainerFile(*containerID, "/layers/config/metadata.toml", "metadata.toml"); err != nil {
		return err
	}
	return nil
}
