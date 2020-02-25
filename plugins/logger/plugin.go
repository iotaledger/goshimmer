package logger

import (
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// define the plugin as a placeholder, so the init methods get executed accordingly
var PLUGIN = node.NewPlugin("Logger", node.Enabled)

func Init() {
	PLUGIN.Events.Init.Trigger(PLUGIN)
}

func init() {
	PLUGIN.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := logger.InitGlobalLogger(config.Node); err != nil {
			panic(err)
		}
	}))
}
