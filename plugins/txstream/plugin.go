package txstream

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	pluginName = "TXStream"
)

var (
	// Plugin is the plugin instance of the TXStream plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin(pluginName, deps, node.Disabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(pluginName)
}

func run(_ *node.Plugin) {
}
