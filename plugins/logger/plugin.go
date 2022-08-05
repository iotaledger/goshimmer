package logger

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
)

// PluginName is the name of the logger plugin.
const PluginName = "Logger"

// Plugin is the plugin instance of the logger plugin.
var Plugin = node.NewPlugin(PluginName, nil, node.Enabled)

// Init triggers the Init event.
func Init(container *dig.Container) {
	Plugin.Events.Init.Trigger(&node.InitEvent{
		Plugin:    Plugin,
		Container: container,
	})
}

func init() {
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Invoke(func(config *configuration.Configuration) {
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
