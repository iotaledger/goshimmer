package indexer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm/indexer"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Indexer"

var (
	Plugin *node.Plugin

	deps       = new(dependencies)
	pluginDeps = new(pluginDependencies)
)

type dependencies struct {
	dig.In

	Protocol *protocol.Protocol
}

type pluginDependencies struct {
	dig.In

	Protocol *protocol.Protocol
	Indexer  *indexer.Indexer
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func provide(deps dependencies) (i *indexer.Indexer) {
	// TODO: needs to consider switching of instance/ledger in the future
	// TODO: load snapshot / attach to events from snapshot loading
	return indexer.New(func() *ledger.Ledger {
		return deps.Protocol.Engine().Ledger
	})
}

func configure(*node.Plugin) {
	deps.Protocol.Events.Engine.Ledger.OutputCreated.Attach(event.NewClosure(pluginDeps.Indexer.OnOutputCreated))
	deps.Protocol.Events.Engine.Ledger.OutputSpent.Attach(event.NewClosure(pluginDeps.Indexer.OnOutputSpentRejected))
	deps.Protocol.Events.Engine.Ledger.OutputRejected.Attach(event.NewClosure(pluginDeps.Indexer.OnOutputSpentRejected))
}
