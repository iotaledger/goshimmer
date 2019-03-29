package server

import (
    "github.com/iotaledger/goshimmer/packages/node"
)

func Configure(plugin *node.Plugin) {
    ConfigureUDPServer(plugin)
    ConfigureTCPServer(plugin)
}

func Run(plugin *node.Plugin) {
    RunUDPServer(plugin)
    RunTCPServer(plugin)
}

func Shutdown(plugin *node.Plugin) {
    ShutdownUDPServer(plugin)
    ShutdownTCPServer(plugin)
}
