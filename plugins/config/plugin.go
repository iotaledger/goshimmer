package config

import (
	"github.com/iotaledger/hive.go/node"
)

// define the plugin as a placeholder, so the init methods get executed accordingly
var PLUGIN = node.NewPlugin("Config", node.Enabled, run)

func run(ctx *node.Plugin) {
	// do nothing; everything is handled in the init method
}
