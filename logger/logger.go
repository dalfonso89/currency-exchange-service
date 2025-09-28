package logger

import (
	"github.com/sirupsen/logrus"
)

// Fields represents key-value pairs for structured logging
type Fields map[string]interface{}

// Logger defines the interface for logging operations
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	WithFields(fields Fields) Logger
}

// LogrusLogger wraps logrus.Logger to implement our Logger interface
type LogrusLogger struct {
	*logrus.Logger
}

// WithFields returns a new logger with fields
func (l *LogrusLogger) WithFields(fields Fields) Logger {
	return &LogrusLogger{Logger: l.Logger.WithFields(logrus.Fields(fields)).Logger}
}

// ensure LogrusLogger implements Logger interface
var _ Logger = (*LogrusLogger)(nil)

// New creates a new logger instance
func New(level string) Logger {
	logrusLogger := logrus.New()
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	// Set log level
	switch level {
	case "debug":
		logrusLogger.SetLevel(logrus.DebugLevel)
	case "info":
		logrusLogger.SetLevel(logrus.InfoLevel)
	case "warn":
		logrusLogger.SetLevel(logrus.WarnLevel)
	case "error":
		logrusLogger.SetLevel(logrus.ErrorLevel)
	default:
		logrusLogger.SetLevel(logrus.InfoLevel)
	}

	return &LogrusLogger{Logger: logrusLogger}
}

// NewLogrusLogger creates a logger from an existing logrus.Logger instance
func NewLogrusLogger(logrusLogger *logrus.Logger) Logger {
	return &LogrusLogger{Logger: logrusLogger}
}
