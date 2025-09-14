package util

import (
	"fmt"
	"log"
	"log/slog"
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
	w.logger.Info(string(p))
	return len(p), nil
}