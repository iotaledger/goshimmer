package retainer

import (
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	protocolplugin "github.com/iotaledger/goshimmer/plugins/protocol"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// PluginName is the name of the spammer plugin.
const PluginName = "Retainer"

var (
	// Plugin is the plugin instance of the spammer plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Protocol *protocol.Protocol
	Retainer *retainer.Retainer
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)

	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(createRetainer); err != nil {
			Plugin.Panic(err)
		}
	})
}

func configure(*node.Plugin) {
	deps.Protocol.Events.Engine.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
		deps.Retainer.PruneUntilSlot(index - slot.Index(Parameters.PruningThreshold))
	}, event.WithWorkerPool(deps.Retainer.BlockWorkerPool()))
}

func createRetainer(p *protocol.Protocol) *retainer.Retainer {
	var dbProvider database.DBProvider
	if protocolplugin.DatabaseParameters.InMemory {
		dbProvider = database.NewMemDB
	} else {
		dbProvider = database.NewDB
	}

	return retainer.NewRetainer(workerpool.NewGroup("Retainer"), p, database.NewManager(protocol.DatabaseVersion, database.WithGranularity(Parameters.DBGranularity), database.WithMaxOpenDBs(Parameters.MaxOpenDBs), database.WithDBProvider(dbProvider), database.WithBaseDir(Parameters.Directory)))
}
