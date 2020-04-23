package server

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgServerPort defines the config flag of the analysis server TCP port.
	CfgServerPort = "analysis.server.port"
)

func init() {
	flag.Int(CfgServerPort, 0, "tcp port for incoming analysis packets")
}
