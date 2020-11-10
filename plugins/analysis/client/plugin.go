package client

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/mana"
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
	// CfgMaxManaEventBufferSize defines the max size of mana event buffer.
	CfgMaxManaEventBufferSize = "analysis.client.maxManaEventsBufferSize"
	// defines the report interval of the reporting in seconds.
	reportIntervalSec = 5
	// voteContextChunkThreshold defines the maximum number of vote context to fit into an FPC update.
	voteContextChunkThreshold = 50
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:21888", "tcp server for collecting analysis information")
	flag.Int(CfgMaxManaEventBufferSize, 100, "max size of mana event buffer")
}

var (
	// plugin is the plugin instance of the analysis client plugin.
	plugin       *node.Plugin
	once         sync.Once
	log          *logger.Logger
	conn         *Connector
	pledgeEvents []*mana.PledgedEvent
	revokeEvents []*mana.RevokedEvent
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

		mana.Events().Pledged.Attach(events.NewClosure(func(ev *mana.PledgedEvent) {
			if len(pledgeEvents) >= config.Node().GetInt(CfgMaxManaEventBufferSize) {
				pledgeEvents = pledgeEvents[1:]
			}
			pledgeEvents = append(pledgeEvents, ev)
		}))

		mana.Events().Revoked.Attach(events.NewClosure(func(ev *mana.RevokedEvent) {
			if len(revokeEvents) >= config.Node().GetInt(CfgMaxManaEventBufferSize) {
				revokeEvents = revokeEvents[1:]
			}
			revokeEvents = append(revokeEvents, ev)
		}))

		ticker := time.NewTicker(reportIntervalSec * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return

			case <-ticker.C:
				sendHeartbeat(conn, createHeartbeat())
				sendMetricHeartbeat(conn, createMetricHeartbeat())
				manaHb, err := createManaHeartbeat()
				if err != nil {
					log.Errorf("error creating mana heartbeat: %s", err)
				}
				sendManaHeartbeat(conn, manaHb)
			}
		}
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// GetPledgeEvents gets the cached mana pledge events and clears the cache afterwards.
func GetPledgeEvents() []mana.PledgedEvent {
	var evs []mana.PledgedEvent
	for _, ev := range pledgeEvents {
		evs = append(evs, *ev)
	}
	// clear buffer
	pledgeEvents = []*mana.PledgedEvent{}
	return evs
}

// GetRevokeEvents gets the cached mana revoke events and clears the cache afterwards.
func GetRevokeEvents() []mana.RevokedEvent {
	var evs []mana.RevokedEvent
	for _, ev := range revokeEvents {
		evs = append(evs, *ev)
	}
	// clear buffer
	revokeEvents = []*mana.RevokedEvent{}
	return evs
}
