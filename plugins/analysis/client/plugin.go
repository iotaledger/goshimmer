package client

import (
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
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

type dependencies struct {
	dig.In

	Local     *peer.Local
	Config    *configuration.Configuration
	Voter     vote.DRNGRoundBasedVoter
	Selection *selection.Protocol
}

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:21888", "tcp server for collecting analysis information")
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
}

var (
	// Plugin is the plugin instance of the analysis client plugin.
	Plugin *node.Plugin
	log    *logger.Logger
	conn   *Connector
	deps   dependencies
)

func configure(_ *node.Plugin) {
	dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})
}

func run(_ *node.Plugin) {
	finalized = make(map[string]opinion.Opinion)
	log = logger.NewLogger(PluginName)
	conn = NewConnector("tcp", deps.Config.String(CfgServerAddress))

	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		conn.Start()
		defer conn.Stop()

		onFinalizedClosure := events.NewClosure(onFinalized)
		deps.Voter.Events().Finalized.Attach(onFinalizedClosure)
		defer deps.Voter.Events().Finalized.Detach(onFinalizedClosure)

		onRoundExecutedClosure := events.NewClosure(onRoundExecuted)
		deps.Voter.Events().RoundExecuted.Attach(onRoundExecutedClosure)
		defer deps.Voter.Events().RoundExecuted.Detach(onRoundExecutedClosure)

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
