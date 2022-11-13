package build

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/buildpacks/pack/pkg/client"
	"github.com/buildpacks/pack/pkg/image"
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
	params, err := b.aws.GetParametersByPath(paths[0])
	if err != nil {
		return nil, err
	}
	if len(paths) == 1 {
		return params, nil
	}
	// overlay vars from additional paths (for review apps)
	for _, path := range paths[1:] {
		p, err := b.aws.GetParametersByPath(path)
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
	cl, err := client.NewClient(client.WithLogger(b.logger))
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
	err = cl.Build(b.Context, client.BuildOptions{
		AppPath:    ".",
		Builder:    appJson.GetBuilders()[0],
		Buildpacks: appJson.GetBuildpacks(),
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
	PrintEndMarker("build")
	c, err := containers.New(b.Context)
	if err != nil {
		return err
	}
	if err = c.PullImage(imageName, b.logger); err != nil {
		return err
	}
	containerID, err := c.CreateContainer(imageName, b.Appname)
	if err != nil {
		return err
	}
	reader, err := c.GetContainerFile(*containerID, "/layers/config/metadata.toml")
	if err != nil {
		return err
	}
	return b.state.WriteMetadataToml(reader)
}
