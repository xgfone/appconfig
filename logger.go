package main

import (
	"fmt"
	"io"
	"os"

	"github.com/xgfone/go-tools/log2"
	loghandler "github.com/xgfone/go-tools/log2/handler"
	"github.com/xgfone/log"
)

var logger = log.Std

func initLogger(filename, level string) {
	var writer io.Writer = os.Stderr
	if filename != "" {
		writer = loghandler.NewSizedRotatingFile(filename, 1024*1024*100, 100)
	}
	logger = log.New(writer, "", log.Ldefault)
	logger.SetOutputLevel(log.NameToLevel(level))
	log.Std = logger

	log2.ErrorF = logger.Errorf
	log2.DebugF = logger.Infof
}

func printLog(err error, format string, args ...interface{}) {
	if err == nil {
		logger.Output("", log.Linfo, 2, fmt.Sprintf(format, args...))
	} else {
		args = append(args, err)
		logger.Output("", log.Lerror, 2, fmt.Sprintf(format+": %s", args...))
	}
}
