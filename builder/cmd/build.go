package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Run prebuild steps",
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := build.New(cmd.Context(), logger)
		if err != nil {
			logger.Error("Failed to create build")
			return err
		}
		if err = b.RunBuild(); err != nil {
			logger.Error("Failed to create build")
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
