package main

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/analysis"
    "github.com/iotaledger/goshimmer/plugins/autopeering"
    "github.com/iotaledger/goshimmer/plugins/cli"
    "github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
    "github.com/iotaledger/goshimmer/plugins/statusscreen"
)

func main() {
    node.Run(
        cli.PLUGIN,
        autopeering.PLUGIN,
        statusscreen.PLUGIN,
        gracefulshutdown.PLUGIN,
        analysis.PLUGIN,
    )
}
