package webinterface

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgBindAddress defines the binding address of the analysis web interface.
	CfgBindAddress = "analysis.webInterface.bindAddress"
	// CfgDev defines whether to run the web interface in dev mode.
	CfgDev = "analysis.webInterface.dev"
)

func init() {
	flag.String(CfgBindAddress, "0.0.0.0:80", "the bind address for the web API")
	flag.Bool(CfgDev, false, "whether the analysis server visualizer is running dev mode")
}
