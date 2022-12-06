package cmd

import (
	"fmt"
	"os"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
	"github.com/spf13/cobra"
)

var DebugLogging = true
var logger = logging.NewLogWithWriters(color.Stdout(), color.Stderr())

var rootCmd = &cobra.Command{
	Use:   "ap-build",
	Short: "ap-build handles the build pipeline for AppPack",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func Execute() {
	logger.WantVerbose(DebugLogging)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
