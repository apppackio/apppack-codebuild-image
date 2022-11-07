package build

import (
	"encoding/json"
	"io/ioutil"

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

type AppJson struct {
	Buildpacks   []Buildpack            `json:"buildpacks"`
	Stack        string                 `json:"stack"`
	Scripts      map[string]string      `json:"scripts"`
	Environments map[string]Environment `json:"environments"`
}

func (a *AppJson) GetBuildpacks() []string {
	var buildpacks []string
	for _, bp := range a.Buildpacks {
		buildpacks = append(buildpacks, bp.URL)
	}
	return buildpacks
}

func ParseAppJson() (*AppJson, error) {
	content, err := ioutil.ReadFile("app.json")
	if err != nil {
		log.Debug().Err(err).Msg("failed to read app.json")
		content = []byte("{}")
	}
	// set defaults
	appJson := AppJson{
		Stack: "heroku-22",
	}
	err = json.Unmarshal(content, &appJson)
	if err != nil {
		log.Error().Msg("failed to parse app.json")
		return nil, err
	}
	return &appJson, nil
}

func (a *AppJson) GetBuilders() []string {
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
