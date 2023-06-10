package log

import (
	"github.com/go-extras/go-kit/logger"
	"github.com/sirupsen/logrus"
)

type Fields = logrus.Fields
type FieldLogger = logger.FieldLogger[logrus.Fields, *logrus.Entry]
type TraceFieldLogger = logger.TraceFieldLogger[logrus.Fields, *logrus.Entry]

var log FieldLogger = logrus.StandardLogger()

func SetLogger(l FieldLogger) {
	log = l
}

func Printf(format string, args ...any) {
	log.Printf(format, args...)
}

func Print(args ...any) {
	log.Print(args...)
}

func Fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}

func Panicf(format string, args ...any) {
	log.Panicf(format, args...)
}

func Fatal(args ...any) {
	log.Fatal(args...)
}

func Panic(args ...any) {
	log.Panic(args...)
}

func Debugf(format string, args ...any) {
	log.Debugf(format, args...)
}

func Infof(format string, args ...any) {
	log.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	log.Warnf(format, args...)
}

func Warningf(format string, args ...any) {
	log.Warningf(format, args...)
}

func Errorf(format string, args ...any) {
	log.Errorf(format, args...)
}

func Debug(args ...any) {
	log.Debug(args...)
}

func Info(args ...any) {
	log.Info(args...)
}

func Warn(args ...any) {
	log.Warn(args...)
}

func Warning(args ...any) {
	log.Warning(args...)
}

func Error(args ...any) {
	log.Error(args...)
}

func Tracef(format string, args ...any) {
	switch tlog := (any)(log).(type) {
	case TraceFieldLogger:
		tlog.Tracef(format, args...)
	default:
		log.Debugf(format, args...)
	}
}

func Trace(args ...any) {
	switch tlog := (any)(log).(type) {
	case TraceFieldLogger:
		tlog.Trace(args...)
	default:
		log.Debug(args...)
	}
}

func WithField(key string, value any) FieldLogger {
	return log.WithField(key, value)
}

func WithFields(fields Fields) FieldLogger {
	return log.WithFields(fields)
}

func WithError(err error) FieldLogger {
	return log.WithError(err)
}
