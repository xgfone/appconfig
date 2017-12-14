package logger

import (
	"github.com/golang/glog"
	"github.com/xgfone/go-tools/log2"
)

func init() {
	glog.MaxSize = 1024 * 1024 * 512
	log2.ErrorF = Error
	log2.DebugF = Debug
}

// Debug outputs the debug information.
func Debug(format string, args ...interface{}) {
	glog.V(0).Infof(format, args...)
}

// Info outputs the info information.
func Info(format string, args ...interface{}) {
	glog.Infof(format, args...)
}

// Warn outputs the warning information.
func Warn(format string, args ...interface{}) {
	glog.Warningf(format, args...)
}

// Error outputs the error information.
func Error(format string, args ...interface{}) {
	glog.Errorf(format, args...)
}

// Fatal outputs the fatal information, then the program exits.
func Fatal(format string, args ...interface{}) {
	glog.Fatalf(format, args...)
}
