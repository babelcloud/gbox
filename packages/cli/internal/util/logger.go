package util

import (
	"fmt"
	"log"
	"log/slog"
	"strings"
)

// Logger wraps slog and provides traditional log.Printf style methods
type Logger struct {
	slogLogger *slog.Logger
}

// GetCompatLogger returns a logger that provides both slog and traditional log.Printf style methods
func GetCompatLogger() *Logger {
	return &Logger{
		slogLogger: GetLogger(),
	}
}

// Printf provides log.Printf compatibility while using slog internally
func (l *Logger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.slogLogger.Info(msg)
}

// Debugf logs at debug level
func (l *Logger) Debugf(format string, v ...interface{}) {
	if IsVerbose() {
		msg := fmt.Sprintf(format, v...)
		l.slogLogger.Debug(msg)
	}
}

// Errorf logs at error level  
func (l *Logger) Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.slogLogger.Error(msg)
}

// Warnf logs at warn level
func (l *Logger) Warnf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.slogLogger.Warn(msg)
}

// Infof logs at info level
func (l *Logger) Infof(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.slogLogger.Info(msg)
}

// SetupGlobalLogger replaces the standard log package logger
func SetupGlobalLogger() {
	logger := GetCompatLogger()
	log.SetOutput(&logWriter{logger: logger.slogLogger})
}

type logWriter struct {
	logger *slog.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// Split by lines and log each non-empty line to avoid empty log entries
	content := strings.TrimSpace(string(p))
	if content != "" {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				w.logger.Info(line)
			}
		}
	}
	return len(p), nil
}

// PrefixLogWriter implements io.Writer for logging with a prefix
type PrefixLogWriter struct {
	prefix string
	logger *slog.Logger
}

// NewPrefixLogWriter creates a new PrefixLogWriter
func NewPrefixLogWriter(prefix string) *PrefixLogWriter {
	return &PrefixLogWriter{
		prefix: prefix,
		logger: GetLogger(),
	}
}

func (w *PrefixLogWriter) Write(p []byte) (n int, err error) {
	// Split by lines and log each non-empty line
	lines := strings.Split(strings.TrimSpace(string(p)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Filter out verbose scrcpy DEBUG and VERBOSE messages
			if w.prefix == "[scrcpy-out]" && (strings.Contains(line, "DEBUG:") || strings.Contains(line, "VERBOSE:")) {
				// Log DEBUG/VERBOSE messages at debug level instead of info
				if IsVerbose() {
					w.logger.Debug(w.prefix+" "+line)
				}
			} else {
				w.logger.Info(w.prefix+" "+line)
			}
		}
	}
	return len(p), nil
}