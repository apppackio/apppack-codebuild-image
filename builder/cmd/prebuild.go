package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/spf13/cobra"
)

var prebuildCmd = &cobra.Command{
	Use:          "prebuild",
	Short:        "Run prebuild steps",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		b, err := build.New(ctx, &logger)
		checkError(err, b.SkipBuild)
		checkError(b.RunPrebuild(ctx), b.SkipBuild)
	},
}

func init() {
	rootCmd.AddCommand(prebuildCmd)
}
