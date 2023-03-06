package indexer

import (
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/hive.go/runtime/event"
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
	Indexer  *indexer.Indexer
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	})
}

func provide(protocol *protocol.Protocol) (i *indexer.Indexer) {
	// TODO: needs to consider switching of instance/ledger in the future
	// TODO: load snapshot / attach to events from snapshot loading
	i = indexer.New(func() ledger.Ledger {
		return protocol.Engine().Ledger
	})

	return i
}

func configure(plugin *node.Plugin) {
	deps.Protocol.Events.Engine.Ledger.OutputCreated.Hook(deps.Indexer.OnOutputCreated, event.WithWorkerPool(plugin.WorkerPool))
	deps.Protocol.Events.Engine.Ledger.OutputSpent.Hook(deps.Indexer.OnOutputSpentRejected, event.WithWorkerPool(plugin.WorkerPool))
	deps.Protocol.Events.Engine.Ledger.OutputRejected.Hook(deps.Indexer.OnOutputSpentRejected, event.WithWorkerPool(plugin.WorkerPool))
}
