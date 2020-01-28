package httpserver

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_BIND_ADDRESS = "analysis.httpServer.bindAddress"
)

func init() {
	flag.String(CFG_BIND_ADDRESS, "0.0.0.0:80", "the bind address for the web API")
}
