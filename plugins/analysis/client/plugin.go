package client

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

const (
	// PluginName is the name of  the analysis client plugin.
	PluginName = "AnalysisClient"
	// CfgServerAddress defines the config flag of the analysis server address.
	CfgServerAddress = "analysis.client.serverAddress"
	// defines the report interval of the reporting.
	reportInterval = 5 * time.Second
)

type dependencies struct {
	dig.In

	Local     *peer.Local
	Config    *configuration.Configuration
	Selection *selection.Protocol `optional:"true"`
}

func init() {
	flag.String(CfgServerAddress, "analysisentry-01.devnet.shimmer.iota.cafe:21888", "tcp server for collecting analysis information")
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, run)
}

var (
	// Plugin is the plugin instance of the analysis client plugin.
	Plugin *node.Plugin
	log    *logger.Logger
	conn   *Connector
	deps   = new(dependencies)
)

func run(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	conn = NewConnector("tcp", deps.Config.String(CfgServerAddress))

	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		conn.Start()
		defer conn.Stop()

		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				if deps.Selection != nil {
					sendHeartbeat(conn, createHeartbeat())
				}
				sendMetricHeartbeat(conn, createMetricHeartbeat())
			}
		}
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
