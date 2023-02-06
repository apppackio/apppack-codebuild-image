package build

import (
	"context"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"
)

type MetadataTomlProcess struct {
	Command []string `toml:"command"`
	Type    string   `toml:"type"`
}

type MetadataToml struct {
	Processes []MetadataTomlProcess `toml:"processes"`
}

func (m MetadataToml) Write(ctx context.Context) error {
	log.Ctx(ctx).Debug().Msg("writing metadata.toml")
	f, err := os.Create("metadata.toml")
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(m)
}
