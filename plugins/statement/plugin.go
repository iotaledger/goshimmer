package statement

import (
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the plugin name of the statement plugin.
	PluginName = "Statement"

	// CfgWaitForStatement is the time in seconds for which the node wait for receiveing the new statement.
	CfgWaitForStatement = "statement.waitForStatement"

	// CfgManaThreshold defines the Mana threshold to accept/write a statement.
	CfgManaThreshold = "statement.manaThreshold"
)

func init() {
	flag.Int(CfgWaitForStatement, 5, "the time in seconds for which the node wait for receiveing the new statement")
	flag.Float64(CfgManaThreshold, 1., "Mana threshold to accept/write a statement")
}

var (
	// plugin is the plugin instance of the statement plugin.
	plugin   *node.Plugin
	once     sync.Once
	log      *logger.Logger
	registry *statement.Registry
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

func Registry() *statement.Registry {
	return registry
}

// configure events
func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	log.Infof("starting node with statement plugin")

	registry = statement.NewRegistry()

	// TODO: check that finalized opinions are also included
	valuetransfers.Voter().Events().RoundExecuted.Attach(events.NewClosure(makeStatement))

	// subscribe to message-layer
	messagelayer.Tangle().Events.MessageSolid.Attach(events.NewClosure(readStatement))

}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Statement", func(shutdownSignal <-chan struct{}) {
		select {
		case <-shutdownSignal:
			return
		}
	}, shutdown.PriorityFPC); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
