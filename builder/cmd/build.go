package cmd

import (
	"github.com/apppackio/codebuild-image/builder/build"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:          "build",
	Short:        "Run build steps",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := logger.WithContext(cmd.Context())
		b, err := build.New(ctx)
		checkError(err, b.SkipBuild)
		checkError(b.RunBuild(), b.SkipBuild)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
