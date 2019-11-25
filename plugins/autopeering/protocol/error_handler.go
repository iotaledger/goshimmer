package protocol

import (
	"net"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/events"
)

func createErrorHandler(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(ip net.IP, err error) {
		plugin.LogDebug("error when communicating with " + ip.String() + ": " + err.Error())
	})
}
