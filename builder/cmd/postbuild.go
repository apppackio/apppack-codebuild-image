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
		ctx := logger.WithContext(cmd.Context())
		b, err := build.New(ctx)
		checkError(err)
		checkError(b.RunPostbuild())
	},
}

func init() {
	rootCmd.AddCommand(postbuildCmd)
}
