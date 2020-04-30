package logger

import (
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the logger plugin.
const PluginName = "Logger"

// Plugin is the plugin instance of the logger plugin.
var Plugin = node.NewPlugin(PluginName, node.Enabled)

// Init triggers the Init event.
func Init() {
	Plugin.Events.Init.Trigger(Plugin)
}

func init() {
	Plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := logger.InitGlobalLogger(config.Node); err != nil {
			panic(err)
		}
	}))
}
