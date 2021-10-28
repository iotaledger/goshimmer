package txstream

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream/server"
	"github.com/iotaledger/goshimmer/packages/txstream/tangleledger"
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
	ledger := tangleledger.New(deps.Tangle)

	bindAddress := Parameters.BindAddress
	log.Debugf("starting TXStream Plugin on %s", bindAddress)
	err := daemon.BackgroundWorker("TXStream worker", func(shutdownSignal <-chan struct{}) {
		err := server.Listen(ledger, bindAddress, log, shutdownSignal)
		if err != nil {
			log.Errorf("failed to start TXStream server: %w", err)
		}
		<-shutdownSignal
	}, shutdown.PriorityTXStream)
	if err != nil {
		log.Errorf("failed to start TXStream daemon: %w", err)
	}
}
