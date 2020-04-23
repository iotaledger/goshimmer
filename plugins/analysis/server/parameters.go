package server

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgServerPort = "analysis.server.port"
)

func init() {
	flag.Int(CfgServerPort, 0, "tcp port for incoming analysis packets")
}
