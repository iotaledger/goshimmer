package webinterface

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgAnalysisWebInterfaceBindAddress defines the binding address of the analysis web interface.
	CfgAnalysisWebInterfaceBindAddress = "analysis.webInterface.bindAddress"
	// CfgAnalysisWebInterfaceDev defines whether to run the web interface in dev mode.
	CfgAnalysisWebInterfaceDev = "analysis.webInterface.dev"
)

func init() {
	flag.String(CfgAnalysisWebInterfaceBindAddress, "0.0.0.0:80", "the bind address for the web API")
	flag.Bool(CfgAnalysisWebInterfaceDev, false, "whether the analysis server visualizer is running dev mode")
}
