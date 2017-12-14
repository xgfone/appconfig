package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/xgfone/appconfig/logger"
	"github.com/xgfone/go-tools/net2/http2"
	"github.com/xgfone/go-tools/signal2"
)

const version = "1.0.0"

type option struct {
	addr  string
	conf  string
	store string

	version bool
}

var (
	opt     option
	handler http.Handler
)

func init() {
	flag.StringVar(&opt.addr, "addr", ":80", "The address to listen to.")
	flag.StringVar(&opt.conf, "conf", "", "The configration information of the backend store.")
	flag.StringVar(&opt.store, "store", "memory", "The backend store type, such as memory, zk, or mysql")

	flag.BoolVar(&opt.version, "version", false, "Print the version and exit.")
}

func main() {
	flag.Parse()
	if opt.version {
		fmt.Println(version)
		return
	}

	if err := InitStore(opt.store, opt.conf); err != nil {
		logger.Error("failed to initialize the backend store [%s]: %s", opt.store, err)
		os.Exit(1)
	}

	// Wrap and handle the signal.
	go signal2.HandleSignal(syscall.SIGTERM, syscall.SIGQUIT)

	// Start HTTP Server.
	http2.ListenAndServe(opt.addr, handler)
}
