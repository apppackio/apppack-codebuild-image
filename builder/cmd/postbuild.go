package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/spf13/cobra"
)

var postbuildCmd = &cobra.Command{
	Use:          "postbuild",
	Short:        "Run postbuild steps",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		b, err := build.New(ctx, &logger)
		checkError(err, b.SkipBuild)
		checkError(b.RunPostbuild(ctx), b.SkipBuild)
	},
}

func init() {
	rootCmd.AddCommand(postbuildCmd)
}
