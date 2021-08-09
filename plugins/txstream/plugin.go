package txstream

import (
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/txstream/server"
	"github.com/iotaledger/goshimmer/packages/txstream/tangleledger"
)

const (
	pluginName = "TXStream"
)

var (
	plugin *node.Plugin
	once   sync.Once

	log *logger.Logger
)

// Plugin returns the plugin instance
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(pluginName, node.Disabled, configPlugin, runPlugin)
	})
	return plugin
}

func configPlugin(plugin *node.Plugin) {
	log = logger.NewLogger(pluginName)
}

func runPlugin(_ *node.Plugin) {
	ledger := tangleledger.New()

	bindAddress := Parameters.BindAddress
	log.Debugf("starting TXStream plugin on %s", bindAddress)
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
