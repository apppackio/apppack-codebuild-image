package build

import (
	"context"
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
)

type Environment struct {
	Scripts map[string]string `json:"scripts"`
	Env     map[string]string `json:"env"`
	Addons  []string          `json:"addons"`
}

type Buildpack struct {
	URL string `json:"url"`
}

type AppJSON struct {
	Buildpacks   []Buildpack            `json:"buildpacks"`
	Stack        string                 `json:"stack"`
	Scripts      map[string]string      `json:"scripts"`
	Environments map[string]Environment `json:"environments"`
	reader       func() ([]byte, error)
	ctx          context.Context
}

const DefaultStack = "heroku-22"

// buildpacks included in builder
var IncludedBuildpacks = map[string][]string{
	"heroku-20": {
		// $ pack builder inspect heroku/buildpacks:20 -o json | jq '.remote_info.buildpacks[].id'
		"heroku/builder-eol-warning",
		"heroku/go",
		"heroku/gradle",
		"heroku/java",
		"heroku/jvm",
		"heroku/maven",
		"heroku/nodejs",
		"heroku/nodejs-corepack",
		"heroku/nodejs-engine",
		"heroku/nodejs-npm-engine",
		"heroku/nodejs-npm-install",
		"heroku/nodejs-pnpm-install",
		"heroku/nodejs-yarn",
		"heroku/php",
		"heroku/procfile",
		"heroku/python",
		"heroku/ruby",
		"heroku/scala",
	},
	"heroku-22": {
		// $ pack builder inspect heroku/builder:22 -o json | jq '.remote_info.buildpacks[].id'
		"heroku/deb-packages",
		"heroku/dotnet",
		"heroku/go",
		"heroku/gradle",
		"heroku/java",
		"heroku/jvm",
		"heroku/maven",
		"heroku/nodejs",
		"heroku/nodejs-corepack",
		"heroku/nodejs-engine",
		"heroku/nodejs-npm-engine",
		"heroku/nodejs-npm-install",
		"heroku/nodejs-pnpm-engine",
		"heroku/nodejs-pnpm-install",
		"heroku/nodejs-yarn",
		"heroku/php",
		"heroku/procfile",
		"heroku/python",
		"heroku/ruby",
		"heroku/sbt",
		"heroku/scala",
	},
}

// contains returns true if the string is in the slice
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// patchBuildpack makes sure buildpacks which are included in the builder are used
// https://github.com/heroku/builder/issues/298
func patchBuildpack(buildpack string, stack string) string {
	CNBBuildpacks, ok := IncludedBuildpacks[stack]
	if !ok {
		return buildpack
	}
	if contains(CNBBuildpacks, buildpack) {
		return "urn:cnb:builder:" + buildpack
	}
	return buildpack
}

func (a *AppJSON) Unmarshal() error {
	content, err := a.reader()
	if err != nil {
		// app.json is optional - default to empty
		log.Ctx(a.ctx).Debug().Err(err).Msg("failed to read app.json")
		content = []byte("{}")
	}
	// set default stack
	a.Stack = DefaultStack
	err = json.Unmarshal(content, &a)
	if err != nil {
		log.Ctx(a.ctx).Error().Err(err).Msg("failed to parse app.json")
		return err
	}
	return nil
}

func ParseAppJson(ctx context.Context) (*AppJSON, error) {
	appJson := AppJSON{
		ctx: ctx,
		reader: func() ([]byte, error) {
			return os.ReadFile("app.json")
		},
	}
	err := appJson.Unmarshal()
	if err != nil {
		return nil, err
	}
	return &appJson, nil
}

// GetBuilders returns the builders from app.json in a format pack can use
// the first item in the list is the builder, followed by the stack image
// the stack image is only used for prefetching, so non-heroku stacks should still work
func (a *AppJSON) GetBuilders() []string {
	if a.Stack == "heroku-18" {
		return []string{"heroku/buildpacks:18", "heroku/heroku:18-cnb"}
	}
	if a.Stack == "heroku-20" {
		return []string{"heroku/buildpacks:20", "heroku/heroku:20-cnb"}
	}
	if a.Stack == "heroku-22" {
		return []string{"heroku/builder:22", "heroku/heroku:22-cnb"}
	}
	return []string{a.Stack}
}

// GetBuildpacks returns the buildpacks from app.json in a format pack can use
func (a *AppJSON) GetBuildpacks() []string {
	var buildpacks []string
	for _, bp := range a.Buildpacks {
		buildpacks = append(buildpacks, patchBuildpack(bp.URL, a.Stack))
	}
	return buildpacks
}

// TestScript returns the test script from app.json
func (a *AppJSON) TestScript() string {
	script, ok := a.Environments["test"].Scripts["test"]
	if !ok {
		return ""
	}
	return script
}

func (a *AppJSON) GetEnv() map[string]string {
	env := map[string]string{}
	for k, v := range a.Environments["test"].Env {
		env[k] = v
	}
	return env
}

// GetTestEnv returns the test environment from app.json
func (a *AppJSON) GetTestEnv() map[string]string {
	env := a.GetEnv()
	env["CI"] = "true"
	return env
}

// GetTestAddons returns the test addons from app.json
func (a *AppJSON) GetTestAddons() []string {
	return a.Environments["test"].Addons
}

// ToApppackToml converts app.json to an apppack.toml
func (a *AppJSON) ToApppackToml() *AppPackToml {
	t := AppPackToml{}
	t.Build.System = "buildpack"
	t.Build.Builder = a.GetBuilders()[0]
	t.Build.Buildpacks = a.GetBuildpacks()
	if a.Scripts["postdeploy"] != "" {
		t.ReviewApp.InitializeCommand = a.Scripts["postdeploy"]
	}
	if a.Scripts["pr-predestroy"] != "" {
		t.ReviewApp.PreDestroyCommand = a.Scripts["pr-predestroy"]
	}
	if a.TestScript() != "" {
		t.Test.Command = a.TestScript()
		for k, v := range a.GetEnv() {
			t.Test.Env = append(t.Test.Env, k+"="+v)
		}
	}
	return &t
}
