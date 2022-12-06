package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/spf13/cobra"
)

var prebuildCmd = &cobra.Command{
	Use:   "prebuild",
	Short: "Run prebuild steps",
	RunE: func(cmd *cobra.Command, args []string) error {
		pb, err := build.New(cmd.Context(), logger)
		if err != nil {
			logger.Error("Failed to create prebuild")
			return err
		}
		if err = pb.RunPrebuild(); err != nil {
			logger.Error("prebuild failed")
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(prebuildCmd)
}
