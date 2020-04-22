package client

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgServerAddress = "analysis.client.serverAddress"
	ReportInterval   = 5
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}
