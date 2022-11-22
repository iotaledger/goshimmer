package network

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Network"

var (
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Network *network.Protocol
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configureLogging)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configureLogging(*node.Plugin) {
	deps.Network.Events.Error.Attach(event.NewClosure(func(errorEvent *network.ErrorEvent) {
		Plugin.LogErrorf("Error in Network: %s (source: %s)", errorEvent.Error, errorEvent.Source.String())
	}))

}

func provide(p2pManager *p2p.Manager) *network.Protocol {
	return network.NewProtocol(p2pManager)
}
