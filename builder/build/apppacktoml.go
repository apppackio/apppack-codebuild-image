package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/apppackio/codebuild-image/builder/filesystem"
	"github.com/rs/zerolog/log"
)

const (
	DockerBuildSystemKeyword    = "dockerfile"
	BuildpackBuildSystemKeyword = "buildpack"
)

type AppPackTomlBuild struct {
	System     string   `toml:"system,omitempty"`
	Buildpacks []string `toml:"buildpacks,omitempty"`
	Builder    string   `toml:"builder,omitempty"`
	Dockerfile string   `toml:"dockerfile,omitempty"`
}

type AppPackTomlTest struct {
	Command string   `toml:"command,omitempty"`
	Env     []string `toml:"env,omitempty"`
}

type AppPackTomlDeploy struct {
	ReleaseCommand string `toml:"release_command,omitempty"`
}

type AppPackTomlReviewApp struct {
	InitializeCommand string `toml:"initialize_command,omitempty"`
	PreDestroyCommand string `toml:"pre_destroy_command,omitempty"`
}

type AppPackTomlService struct {
	Command string `toml:"command,omitempty"`
}

type AppPackToml struct {
	Build     AppPackTomlBuild              `toml:"build,omitempty"`
	Test      AppPackTomlTest               `toml:"test,omitempty"`
	Deploy    AppPackTomlDeploy             `toml:"deploy,omitempty"`
	ReviewApp AppPackTomlReviewApp          `toml:"review_app,omitempty"`
	Services  map[string]AppPackTomlService `toml:"services,omitempty"`
}

func (a AppPackToml) UseBuildpacks() bool {
	return a.Build.System == BuildpackBuildSystemKeyword || a.Build.System == ""
}

func (a AppPackToml) UseDockerfile() bool {
	return a.Build.System == DockerBuildSystemKeyword
}

func (a AppPackToml) Validate() error {
	if !a.UseBuildpacks() && !a.UseDockerfile() {
		return errors.New("apppack.toml: [build] unknown value for system")
	}
	if a.UseBuildpacks() && len(a.Services) > 0 {
		return errors.New("apppack.toml: [build] buildpacks cannot be used with services -- use Procfile instead")
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
		return errors.New("apppack.toml: [services] no web service defined")
	}
	return nil
}

func (a AppPackToml) GetTestEnv() map[string]string {
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

func ParseAppPackToml(ctx context.Context) (*AppPackToml, error) {
	var config AppPackToml
	// if the file doesn't exist, just return an empty config
	filename := filesystem.GetAppPackTomlFilename()
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Ctx(ctx).Debug().Msg(filename + " not found")
		return &config, nil
	}
	if _, err := toml.DecodeFile(filename, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (a AppPackToml) Write(ctx context.Context) (err error) {
	filename := filesystem.GetAppPackTomlFilename()
	log.Ctx(ctx).Debug().Msg("writing " + filename)
	f, err := os.Create(filename) //nolint:gosec // G304: the path comes from APPPACK_TOML, set by whoever configured the pipeline, and writing it is the point of this function.
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return toml.NewEncoder(f).Encode(a)
}
