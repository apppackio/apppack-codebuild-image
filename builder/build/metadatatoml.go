package build

import (
	"context"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/apppackio/codebuild-image/builder/build/shlex"
	"github.com/rs/zerolog/log"
)

type BuildpackMetadataTomlProcess struct {
	Command     []string `toml:"command"`
	Type        string   `toml:"type"`
	BuildpackID string   `toml:"buildpack_id"`
}

type BuildpackMetadataToml struct {
	Processes []BuildpackMetadataTomlProcess `toml:"processes"`
}

func (m *BuildpackMetadataToml) ToApppackServices() map[string]AppPackTomlService {
	services := map[string]AppPackTomlService{}
	for _, process := range m.Processes {
		if process.Type == "release" {
			continue
		}
		if process.BuildpackID == "heroku/ruby" && (process.Type == "rake" || process.Type == "console") {
			continue
		}
		// these seem like they are always a single command, so we can just use the first element
		if len(process.Command) == 1 {
			services[process.Type] = AppPackTomlService{Command: process.Command[0]}
		} else {
			services[process.Type] = AppPackTomlService{Command: shlex.Join(process.Command)}
		}
	}
	return services
}

func ParseBuildpackMetadataToml(ctx context.Context) (*BuildpackMetadataToml, error) {
	var config BuildpackMetadataToml
	// if the file doesn't exist, just return an empty config
	if _, err := os.Stat("metadata.toml"); os.IsNotExist(err) {
		log.Ctx(ctx).Debug().Msg("metadata.toml not found")
		return &config, nil
	}
	if _, err := toml.DecodeFile("metadata.toml", &config); err != nil {
		return nil, err
	}
	return &config, nil
}
