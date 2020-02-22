package logger

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// define the plugin as a placeholder, so the init methods get executed accordingly
var PLUGIN = node.NewPlugin("Logger", node.Enabled)

func Load() {
	// do nothing - the config get's initialized during init()
}

func init() {
	if err := logger.InitGlobalLogger(config.Node); err != nil {
		panic(err)
	}
}
