package protocol

import (
	"net"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
)

func createErrorHandler(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(ip net.IP, err error) {
		plugin.LogDebug("error when communicating with " + ip.String() + ": " + err.Error())
	})
}
