package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/spf13/cobra"
)

var postbuildCmd = &cobra.Command{
	Use:          "postbuild",
	Short:        "Run prebuild steps",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		pb, err := build.New(cmd.Context(), logger)
		if err != nil {
			logger.Error("Failed to create postbuild")
			return err
		}
		if err = pb.RunPostbuild(); err != nil {
			logger.Error("postbuild failed")
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(postbuildCmd)
}
