package logger

import (
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
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
	configuration.BindParameters(Parameters, "logger")

	Plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := dependencyinjection.Container.Invoke(func(config *configuration.Configuration) {
			if err := logger.InitGlobalLogger(config); err != nil {
				panic(err)
			}
		}); err != nil {
			Plugin.LogError(err)
		}

		// enable logging for the daemon
		daemon.DebugEnabled(true)
	}))
}
