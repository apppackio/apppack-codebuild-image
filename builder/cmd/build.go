package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Run prebuild steps",
	Run: func(cmd *cobra.Command, args []string) {
		pb, err := build.New()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create build")
		}
		err = pb.RunBuild()
		if err != nil {
			log.Fatal().Err(err).Msg("build failed")
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
