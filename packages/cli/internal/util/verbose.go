package util

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGreen  = "\033[32m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
)

// PrettyHandler is a custom slog handler that provides colorized, human-readable output
type PrettyHandler struct {
	level slog.Level
}

// NewPrettyHandler creates a new PrettyHandler
func NewPrettyHandler(level slog.Level) *PrettyHandler {
	return &PrettyHandler{level: level}
}

// Enabled reports whether the handler handles records at the given level
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and outputs the log record
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	// Format time as HH:MM:SS
	timeStr := r.Time.Format("15:04:05")

	// Get level color and symbol
	var levelColor, levelStr string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = ColorGray
		levelStr = "DEBUG"
	case slog.LevelInfo:
		levelColor = ColorBlue
		levelStr = "INFO "
	case slog.LevelWarn:
		levelColor = ColorYellow
		levelStr = "WARN "
	case slog.LevelError:
		levelColor = ColorRed
		levelStr = "ERROR"
	default:
		levelColor = ColorReset
		levelStr = "     "
	}

	// Format message
	msg := r.Message

	// Collect attributes
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		// Format key-value pairs nicely
		value := a.Value.String()
		// Remove quotes from strings for cleaner output
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = strings.Trim(value, `"`)
		}
		attrs = append(attrs, fmt.Sprintf("%s=%s", ColorCyan+a.Key+ColorReset, value))
		return true
	})

	// Build final output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s%s%s [%s%s%s] %s",
		ColorGray, timeStr, ColorReset,
		levelColor, levelStr, ColorReset,
		msg))

	// Add attributes if any
	if len(attrs) > 0 {
		output.WriteString(" ")
		output.WriteString(strings.Join(attrs, " "))
	}

	output.WriteString("\n")
	fmt.Print(output.String())
	return nil
}

// WithAttrs returns a new Handler whose attributes consist of both the receiver's attributes and the arguments
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // For simplicity, not implementing attribute preservation
}

// WithGroup returns a new Handler with the given group appended to the receiver's existing groups
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	return h // For simplicity, not implementing groups
}


// InitLogger initializes the global slog logger with appropriate level
func InitLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	var handler slog.Handler

	// Check if we should use structured logging (for production/server environments)
	if UseStructuredLogging() {
		// Use structured JSON or text handler for production
		opts := &slog.HandlerOptions{Level: level}
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Use pretty handler for development
		handler = NewPrettyHandler(level)
	}

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

// UseStructuredLogging determines whether to use structured logging format
// This is useful for production/server environments where logs need to be parsed
func UseStructuredLogging() bool {
	// Check environment variable
	if env := os.Getenv("LOG_FORMAT"); env != "" {
		switch strings.ToLower(env) {
		case "structured":
			return true
		case "pretty":
			return false
		}
	}

	// Check if running in container or CI environment (production indicators)
	if os.Getenv("CONTAINER") != "" ||
		os.Getenv("CI") != "" ||
		os.Getenv("KUBERNETES_SERVICE_HOST") != "" ||
		os.Getenv("DOCKER_CONTAINER") != "" {
		return true
	}

	// Default to pretty logging for local development (including server command)
	return false
}