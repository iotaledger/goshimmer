package protocol

import (
	"net"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

func createErrorHandler(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(ip net.IP, err error) {
		log.Debugf("error when communicating with %s: %s", ip.String(), err.Error())
	})
}
