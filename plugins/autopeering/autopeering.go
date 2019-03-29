package autopeering

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server"
)

func configure(plugin *node.Plugin) {
    server.Configure(plugin)
    peermanager.Configure(plugin)

    daemon.Events.Shutdown.Attach(func() {
        server.Shutdown(plugin)
        peermanager.Shutdown(plugin)
    })
}

func run(plugin *node.Plugin) {
    server.Run(plugin)
    peermanager.Run(plugin)
}

var PLUGIN = node.NewPlugin("Auto Peering", configure, run)
