package httpserver

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgBindAddress = "analysis.httpServer.bindAddress"
	CfgDev         = "analysis.httpServer.dev"
)

func init() {
	flag.String(CfgBindAddress, "0.0.0.0:80", "the bind address for the web API")
	flag.Bool(CfgDev, false, "whether the analysis server visualizer is running dev mode")
}
