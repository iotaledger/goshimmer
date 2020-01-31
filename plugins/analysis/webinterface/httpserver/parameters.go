package httpserver

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_BIND_ADDRESS = "analysis.httpServer.bindAddress"
	CFG_DEV          = "analysis.httpServer.dev"
)

func init() {
	flag.String(CFG_BIND_ADDRESS, "0.0.0.0:80", "the bind address for the web API")
	flag.Bool(CFG_DEV, false, "whether the analysis server visualizer is running dev mode")
}
