package client

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgServerAddress defines the config flag of the analysis server address.
	CfgServerAddress = "analysis.client.serverAddress"
	// ReportInterval defines the interval of the reporting.
	ReportInterval = 5
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}
