package server

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_SERVER_PORT = "analysis.server.port"
)

func init() {
	flag.Int(CFG_SERVER_PORT, 0, "tcp port for incoming analysis packets")
}
