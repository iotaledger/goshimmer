package indexer

import (
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/hive.go/core/generics/event"
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

func provide(deps dependencies) (i *indexer.Indexer) {
	// TODO: needs to consider switching of instance/ledger in the future
	// TODO: load snapshot / attach to events from snapshot loading
	i = indexer.New(func() *ledger.Ledger {
		return deps.Protocol.Engine().Ledger
	})

	deps.Protocol.Events.Engine.Ledger.OutputCreated.Attach(event.NewClosure(i.OnOutputCreated))
	deps.Protocol.Events.Engine.Ledger.OutputSpent.Attach(event.NewClosure(i.OnOutputSpentRejected))
	deps.Protocol.Events.Engine.Ledger.OutputRejected.Attach(event.NewClosure(i.OnOutputSpentRejected))

	return i
}
