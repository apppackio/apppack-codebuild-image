package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var logger = zerolog.New(zerolog.ConsoleWriter{
	Out:        os.Stdout,
	TimeFormat: "15:04:05",
}).With().Timestamp().Logger()

var rootCmd = &cobra.Command{
	Use:   "apppack-builder",
	Short: "apppack-builder handles the build pipeline for AppPack",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func checkError(err error, postError func() error) {
	if err != nil {
		if err2 := postError(); err2 != nil {
			logger.Error().Err(err2).Msg("Error")
		}
		logger.Fatal().Err(err).Msg("Error")
	}
}

func Execute() {
	zerolog.DisableSampling(true)
	if os.Getenv("APPPACK_DEBUG") != "" {
		logger = logger.Level(zerolog.DebugLevel)
	} else {
		logger = logger.Level(zerolog.InfoLevel)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
