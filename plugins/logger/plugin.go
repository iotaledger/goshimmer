package logger

import (
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the logger plugin.
const PluginName = "Logger"

// Plugin is the plugin instance of the logger plugin.
var Plugin = node.NewPlugin(PluginName, nil, node.Enabled)

// Init triggers the Init event.
func Init() {
	Plugin.Events.Init.Trigger(&node.InitEvent{Plugin: Plugin})
}

func init() {
	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(initEvent *node.InitEvent) {
		if err := initEvent.Container.Invoke(func(config *configuration.Configuration) {
			if err := logger.InitGlobalLogger(config); err != nil {
				panic(err)
			}
		}); err != nil {
			Plugin.Panic(err)
		}

		// enable logging for the daemon
		daemon.DebugEnabled(true)
	}))
}
