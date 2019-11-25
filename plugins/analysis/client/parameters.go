package client

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_SERVER_ADDRESS = "analysis.serverAddress"
)

func init() {
	flag.String(CFG_SERVER_ADDRESS, "159.69.158.51:188", "tcp server for collecting analysis information")
}
