package server

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/udp"
)

func Configure(plugin *node.Plugin) {
    udp.ConfigureServer(plugin)
    tcp.ConfigureServer(plugin)
}

func Run(plugin *node.Plugin) {
    udp.RunServer(plugin)
    tcp.RunServer(plugin)
}

func Shutdown(plugin *node.Plugin) {
    udp.ShutdownUDPServer(plugin)
    tcp.ShutdownServer(plugin)
}
