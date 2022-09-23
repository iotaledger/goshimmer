package indexer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm/indexer"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Indexer"

var (
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Protocol *protocol.Protocol
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func provide() (i *indexer.Indexer) {
	// TODO: needs to consider switching of instance/ledger in the future
	// TODO: load snapshot / attach to events from snapshot loading
	return indexer.New(deps.Protocol.Instance().Engine.Ledger)
}
