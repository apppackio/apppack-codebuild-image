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
		ctx := logger.WithContext(cmd.Context())
		b, err := build.New(ctx)
		checkError(err, b.SkipBuild)
		checkError(b.RunPrebuild(), b.SkipBuild)
	},
}

func init() {
	rootCmd.AddCommand(prebuildCmd)
}
