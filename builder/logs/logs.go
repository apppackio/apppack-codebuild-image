package logs

import (
	"io"
	"os"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
	"github.com/rs/zerolog"
)

type Option func(*logging.LogWithWriters)

func PackLoggerToFileFromZerolog(logger *zerolog.Logger, file *os.File, opts ...Option) (*logging.LogWithWriters, error) {
	return PackLoggerFromZerolog(logger, io.MultiWriter(os.Stdout, file), io.MultiWriter(os.Stderr, file), opts...), nil
}

func PackLoggerFromZerolog(logger *zerolog.Logger, stdout, stderr io.Writer, opts ...Option) *logging.LogWithWriters {
	color.Disable(true)
	packLogger := logging.NewLogWithWriters(stdout, stderr)
	if logger.GetLevel() <= zerolog.DebugLevel {
		packLogger.WantVerbose(true)
		packLogger.WantTime(true)
	}
	for _, opt := range opts {
		opt(packLogger)
	}
	return packLogger
}

func WithQuiet() Option {
	return func(l *logging.LogWithWriters) {
		l.WantQuiet(true)
	}
}
