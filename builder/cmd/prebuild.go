package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var prebuildCmd = &cobra.Command{
	Use:   "prebuild",
	Short: "Run prebuild steps",
	Run: func(cmd *cobra.Command, args []string) {
		pb, err := build.New()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create prebuild")
		}
		err = pb.RunPrebuild()
		if err != nil {
			log.Fatal().Err(err).Msg("prebuild failed")
		}
	},
}

func init() {
	rootCmd.AddCommand(prebuildCmd)
}
