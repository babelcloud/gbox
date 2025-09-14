package util

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

// InitLogger initializes the global slog logger with appropriate level
func InitLogger(verbose bool) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo, // Default level
	}

	if verbose {
		opts.Level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// GetLogger returns the configured logger instance
func GetLogger() *slog.Logger {
	if logger == nil {
		// Fallback initialization with INFO level
		InitLogger(false)
	}
	return logger
}

// IsVerbose checks if verbose mode is enabled by looking at command line arguments
func IsVerbose() bool {
	for _, arg := range os.Args {
		if arg == "--verbose" {
			return true
		}
	}
	return false
}