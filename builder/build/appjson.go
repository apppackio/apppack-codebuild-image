package build

import (
	"encoding/json"
	"os"

	"github.com/buildpacks/pack/pkg/logging"
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
	logger       logging.Logger
}

const DefaultStack = "heroku-20"

// pack builder inspect heroku/builder:22 --output json | jq [.remote_info.buildpacks[].id]
var CNBBuildpacks = []string{
	"heroku/java",
	"heroku/java-function",
	"heroku/jvm",
	"heroku/jvm-function-invoker",
	"heroku/maven",
	"heroku/nodejs",
	"heroku/nodejs-engine",
	"heroku/nodejs-function",
	"heroku/nodejs-function-invoker",
	"heroku/nodejs-npm",
	"heroku/nodejs-yarn",
	"heroku/procfile",
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

// patchBuildpack makes sure CNB buildpacks are preferred over legacy buildpacks
// https://github.com/heroku/builder/issues/298
func patchBuildpack(buildpack string) string {
	if contains(CNBBuildpacks, buildpack) {
		return "urn:cnb:registry:" + buildpack
	}
	return buildpack
}

func (a *AppJSON) Unmarshal() error {
	content, err := a.reader()
	if err != nil {
		// app.json is optional - default to empty
		a.logger.Debugf("failed to read app.json %v", err)
		content = []byte("{}")
	}
	// set default stack
	a.Stack = DefaultStack
	err = json.Unmarshal(content, &a)
	if err != nil {
		a.logger.Error("failed to parse app.json")
		return err
	}
	return nil
}

func ParseAppJson() (*AppJSON, error) {
	appJson := AppJSON{
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
		return []string{"heroku/builder-classic:22", "heroku/heroku:22-cnb"}
	}
	return []string{a.Stack}
}

// GetBuildpacks returns the buildpacks from app.json in a format pack can use
func (a *AppJSON) GetBuildpacks() []string {
	var buildpacks []string
	for _, bp := range a.Buildpacks {
		buildpacks = append(buildpacks, patchBuildpack(bp.URL))
	}
	return buildpacks
}
