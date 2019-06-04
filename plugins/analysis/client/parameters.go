package client

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	SERVER_ADDRESS = parameter.AddString("ANALYSIS/SERVER-ADDRESS", "82.165.29.179:188", "tcp server for collecting analysis information")
)
