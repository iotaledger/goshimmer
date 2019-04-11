package protocol

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "net"
)

func createErrorHandler(plugin *node.Plugin) func(ip net.IP, err error) {
    return func(ip net.IP, err error) {
        plugin.LogDebug("error when communicating with " + ip.String() + ": " + err.Error())
    }
}
