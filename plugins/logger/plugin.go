package logger

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the logger plugin.
const PluginName = "Logger"

var (
	// plugin is the plugin instance of the logger plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled)
	})
	return plugin
}

// Init triggers the Init event.
func Init() {
	plugin.Events.Init.Trigger(plugin)
}

func init() {
	plugin = Plugin()

	initFlags()

	plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := logger.InitGlobalLogger(config.Node()); err != nil {
			panic(err)
		}

		// enable logging for the daemon
		daemon.DebugEnabled(true)
	}))
}
