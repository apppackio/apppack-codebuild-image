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

// buildpack commands are always one item, but just in case, handle joining them
func commandSliceToString(cmd []string) string {
	if len(cmd) == 1 {
		return cmd[0]
	}
	return shlex.Join(cmd)
}

func (m *BuildpackMetadataToml) UpdateAppPackToml(a *AppPackToml) {
	a.Services = make(map[string]AppPackTomlService)
	for _, process := range m.Processes {
		if process.Type == "release" {
			a.Deploy.ReleaseCommand = commandSliceToString(process.Command)
			continue
		}
		if process.BuildpackID == "heroku/ruby" && (process.Type == "rake" || process.Type == "console") {
			continue
		}
		a.Services[process.Type] = AppPackTomlService{Command: commandSliceToString(process.Command)}
	}
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
