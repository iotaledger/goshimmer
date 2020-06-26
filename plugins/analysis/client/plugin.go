package client

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the name of  the analysis client plugin.
	PluginName = "Analysis-Client"
	// CfgServerAddress defines the config flag of the analysis server address.
	CfgServerAddress = "analysis.client.serverAddress"
	// defines the report interval of the reporting in seconds.
	reportIntervalSec = 5
	// voteContextChunkThreshold defines the maximum number of vote context to fit into an FPC update.
	voteContextChunkThreshold = 50
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}

var (
	// plugin is the plugin instance of the analysis client plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
	conn   *Connector
)

// Plugin gets the plugin instance
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, run)
	})
	return plugin
}

func run(_ *node.Plugin) {
	finalized = make(map[string]vote.Opinion)
	log = logger.NewLogger(PluginName)
	conn = NewConnector("tcp", config.Node().GetString(CfgServerAddress))

	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		conn.Start()
		defer conn.Stop()

		onFinalizedClosure := events.NewClosure(onFinalized)
		valuetransfers.Voter().Events().Finalized.Attach(onFinalizedClosure)
		defer valuetransfers.Voter().Events().Finalized.Detach(onFinalizedClosure)

		onRoundExecutedClosure := events.NewClosure(onRoundExecuted)
		valuetransfers.Voter().Events().RoundExecuted.Attach(onRoundExecutedClosure)
		defer valuetransfers.Voter().Events().RoundExecuted.Detach(onRoundExecutedClosure)

		ticker := time.NewTicker(reportIntervalSec * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return

			case <-ticker.C:
				sendHeartbeat(conn, createHeartbeat())
				sendMetricHeartbeat(conn, createMetricHeartbeat())
			}
		}
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
