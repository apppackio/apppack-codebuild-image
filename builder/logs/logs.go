package logs

import (
	"os"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/rs/zerolog"
)

type Option func(*logging.LogWithWriters)

func PackLoggerFromZerolog(logger *zerolog.Logger, opts ...Option) *logging.LogWithWriters {
	packLogger := logging.NewLogWithWriters(os.Stdout, os.Stderr)
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