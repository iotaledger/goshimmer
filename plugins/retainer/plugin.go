package retainer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	protocolplugin "github.com/iotaledger/goshimmer/plugins/protocol"
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
	Server   *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createRetainer); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func createRetainer() *retainer.Retainer {
	// TODO: retainer db needs to be be pruned (create configuration)
	var dbProvider database.DBProvider
	if protocolplugin.DatabaseParameters.InMemory {
		dbProvider = database.NewMemDB
	} else {
		dbProvider = database.NewDB
	}

	return retainer.NewRetainer(deps.Protocol, database.NewManager(protocol.DatabaseVersion, database.WithDBProvider(dbProvider), database.WithBaseDir(Parameters.DBPath)))
}
