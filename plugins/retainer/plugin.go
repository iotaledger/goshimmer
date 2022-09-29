package retainer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol"
)

// PluginName is the name of the spammer plugin.
const PluginName = "Retainer"

var (
	// Plugin is the plugin instance of the spammer plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Protocol *protocol.Protocol
	Server   *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createRetainer); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func createRetainer() *retainer.Retainer {
	// TODO: retainer needs to use protocol instead of engine
	// TODO: retainer db needs to be be pruned (create configuration)
	return retainer.NewRetainer(deps.Protocol, database.NewManager(protocol.DatabaseVersion, database.WithBaseDir(Parameters.DBPath)))
}
