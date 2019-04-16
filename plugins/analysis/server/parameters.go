package server

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
    SERVER_PORT = parameter.AddInt("ANALYSIS/SERVER-PORT", 0, "tcp port for incoming analysis packets")
)
