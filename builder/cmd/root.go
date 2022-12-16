package cmd

import (
	"fmt"
	"os"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
	"github.com/spf13/cobra"
)

var DebugLogging = false
var logger = logging.NewLogWithWriters(color.Stdout(), color.Stderr())

var rootCmd = &cobra.Command{
	Use:   "apppack-builder",
	Short: "apppack-builder handles the build pipeline for AppPack",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&DebugLogging, "debug", "d", true, "enable debug logging")
}

func Execute() {
	logger.WantVerbose(DebugLogging)
	logger.WantTime(DebugLogging)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
