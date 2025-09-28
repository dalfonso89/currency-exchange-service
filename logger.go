package main

import (
	"github.com/sirupsen/logrus"
)

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
	WithFields(fields logrus.Fields) Logger
}

// logrusLogger wraps logrus.Logger to implement our Logger interface
type logrusLogger struct {
	*logrus.Logger
}

// WithFields returns a new logger with fields
func (l *logrusLogger) WithFields(fields logrus.Fields) Logger {
	return &logrusLogger{Logger: l.Logger.WithFields(fields).Logger}
}

// ensure logrusLogger implements Logger interface
var _ Logger = (*logrusLogger)(nil)
