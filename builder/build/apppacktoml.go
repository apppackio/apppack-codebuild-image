package build

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"
)

const (
	DockerBuildSystemKeyword    = "dockerfile"
	BuildpackBuildSystemKeyword = "buildpack"
)

type AppPackTomlBuild struct {
	System     string   `toml:"system"`
	Buildpacks []string `toml:"buildpacks"`
	Builder    string   `toml:"builder"`
	Dockerfile string   `toml:"dockerfile"`
}

type AppPackTomlTest struct {
	Command string   `toml:"command"`
	Env     []string `toml:"env"`
}

type AppPackTomlDeploy struct {
	ReleaseCommand string `toml:"release_command"`
}

type AppPackTomlServices struct {
	Command string `toml:"command"`
}

type AppPackToml struct {
	Build    AppPackTomlBuild               `toml:"build"`
	Test     AppPackTomlTest                `toml:"test"`
	Deploy   AppPackTomlDeploy              `toml:"deploy"`
	Services map[string]AppPackTomlServices `toml:"services"`
}

func (a AppPackToml) UseBuildpacks() bool {
	return a.Build.System == BuildpackBuildSystemKeyword || a.Build.System == ""
}

func (a AppPackToml) UseDockerfile() bool {
	return a.Build.System == DockerBuildSystemKeyword
}

func (a AppPackToml) Validate() error {
	if !a.UseBuildpacks() && !a.UseDockerfile() {
		return fmt.Errorf("apppack.toml: [build] unknown value for system")
	}
	if a.UseBuildpacks() && len(a.Services) > 0 {
		return fmt.Errorf("apppack.toml: [build] buildpacks cannot be used with services -- use Procfile instead")
	}
	for _, e := range a.Test.Env {
		if !strings.Contains(e, "=") {
			return fmt.Errorf("apppack.toml: [test] env %s is not in KEY=VALUE format", e)
		}
	}
	// all validation below is for dockerfile builds
	if !a.UseDockerfile() {
		return nil
	}
	hasWeb := false
	for s := range a.Services {
		if s == "web" {
			hasWeb = true
		}
		if a.Services[s].Command == "" {
			return fmt.Errorf("apppack.toml: [services] service %s has no command", s)
		}
	}
	if !hasWeb {
		return fmt.Errorf("apppack.toml: [services] no web service defined")
	}
	return nil
}

func (a *AppPackToml) GetTestEnv() map[string]string {
	env := map[string]string{
		"CI": "true",
	}
	for e := range a.Test.Env {
		kv := strings.SplitN(a.Test.Env[e], "=", 2)
		if len(kv) == 2 {
			env[kv[0]] = kv[1]
		}
	}
	return env
}

func (a *AppPackToml) ToMetadataToml() MetadataToml {
	var metadataToml MetadataToml
	for s := range a.Services {
		metadataToml.Processes = append(metadataToml.Processes, MetadataTomlProcess{
			Type:    s,
			Command: []string{a.Services[s].Command},
		})
	}
	if a.Deploy.ReleaseCommand != "" {
		metadataToml.Processes = append(metadataToml.Processes, MetadataTomlProcess{
			Type:    "release",
			Command: []string{a.Deploy.ReleaseCommand},
		})
	}
	return metadataToml
}

func ParseAppPackToml(ctx context.Context) (*AppPackToml, error) {
	var config AppPackToml
	// if the file doesn't exist, just return an empty config
	if _, err := os.Stat("apppack.toml"); os.IsNotExist(err) {
		log.Ctx(ctx).Debug().Msg("apppack.toml not found")
		return &config, nil
	}
	if _, err := toml.DecodeFile("apppack.toml", &config); err != nil {
		return nil, err
	}
	return &config, nil
}
