package client

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	SERVER_ADDRESS = parameter.AddString("ANALYSIS/SERVER-ADDRESS", "159.69.158.51:188", "tcp server for collecting analysis information")
)
