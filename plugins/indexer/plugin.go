package indexer

import (
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
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
	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	})
}

func provide(deps dependencies) (i *indexer.Indexer) {
	// TODO: needs to consider switching of instance/ledger in the future
	// TODO: load snapshot / attach to events from snapshot loading
	i = indexer.New(func() *ledger.Ledger {
		return deps.Protocol.Engine().Ledger
	})

	wp := workerpool.New(PluginName, 1)
	deps.Protocol.Events.Engine.Ledger.OutputCreated.Hook(i.OnOutputCreated, event.WithWorkerPool(wp))
	deps.Protocol.Events.Engine.Ledger.OutputSpent.Hook(i.OnOutputSpentRejected, event.WithWorkerPool(wp))
	deps.Protocol.Events.Engine.Ledger.OutputRejected.Hook(i.OnOutputSpentRejected, event.WithWorkerPool(wp))

	return i
}
