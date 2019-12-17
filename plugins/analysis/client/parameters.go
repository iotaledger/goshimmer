package client

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_SERVER_ADDRESS = "analysis.serverAddress"
)

func init() {
	flag.String(CFG_SERVER_ADDRESS, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}
