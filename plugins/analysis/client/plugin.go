package client

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/node"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the name of  the analysis client plugin.
	PluginName = "Analysis-Client"
	// CfgServerAddress defines the config flag of the analysis server address.
	CfgServerAddress = "analysis.client.serverAddress"
	// defines the report interval of the reporting in seconds.
	reportIntervalSec = 5
	// maxVoteContext defines the maximum number of vote context to fit into an FPC update
	maxVoteContext = 50
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}

var (
	// Plugin is the plugin instance of the analysis client plugin.
	Plugin      = node.NewPlugin(PluginName, node.Enabled, run)
	log         *logger.Logger
	managedConn *network.ManagedConnection
	connLock    sync.Mutex

	finalized      map[string]vote.Opinion
	finalizedMutex sync.RWMutex
)

func run(_ *node.Plugin) {
	finalized = make(map[string]vote.Opinion)
	log = logger.NewLogger(PluginName)

	conn, err := net.Dial("tcp", config.Node.GetString(CfgServerAddress))
	if err != nil {
		log.Debugf("Could not connect to reporting server: %s", err.Error())
		return
	}

	managedConn = network.NewManagedConnection(conn)

	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {

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
				sendHeartbeat(managedConn, createHeartbeat())
			}
		}
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onFinalized(id string, opinion vote.Opinion) {
	finalizedMutex.Lock()
	finalized[id] = opinion
	finalizedMutex.Unlock()
}

// EventDispatchers holds the Heartbeat function.
type EventDispatchers struct {
	// Heartbeat defines the Heartbeat function.
	Heartbeat func(heartbeat *packet.Heartbeat)
}

func sendHeartbeat(conn *network.ManagedConnection, hb *packet.Heartbeat) {
	var out strings.Builder
	for _, value := range hb.OutboundIDs {
		out.WriteString(base58.Encode(value))
	}
	var in strings.Builder
	for _, value := range hb.InboundIDs {
		in.WriteString(base58.Encode(value))
	}
	log.Debugw(
		"Heartbeat",
		"nodeID", base58.Encode(hb.OwnID),
		"outboundIDs", out.String(),
		"inboundIDs", in.String(),
	)

	data, err := packet.NewHeartbeatMessage(hb)
	if err != nil {
		log.Info(err, " - heartbeat message skipped")
		return
	}

	connLock.Lock()
	defer connLock.Unlock()
	if _, err = conn.Write(data); err != nil {
		log.Debugw("Error while writing to connection", "Description", err)
	}
}

func createHeartbeat() *packet.Heartbeat {
	// get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	var outboundIDs [][]byte
	var inboundIDs [][]byte

	// get outboundIDs (chosen neighbors)
	outgoingNeighbors := autopeering.Selection().GetOutgoingNeighbors()
	outboundIDs = make([][]byte, len(outgoingNeighbors))
	for i, neighbor := range outgoingNeighbors {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		outboundIDs[i] = neighbor.ID().Bytes()
	}

	// get inboundIDs (accepted neighbors)
	incomingNeighbors := autopeering.Selection().GetIncomingNeighbors()
	inboundIDs = make([][]byte, len(incomingNeighbors))
	for i, neighbor := range incomingNeighbors {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		inboundIDs[i] = neighbor.ID().Bytes()
	}

	return &packet.Heartbeat{OwnID: nodeID, OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
}

func onRoundExecuted(roundStats *vote.RoundStats) {
	// get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	chunks := splitFPCVoteContext(roundStats.ActiveVoteContexts)

	connLock.Lock()
	defer connLock.Unlock()

	for _, chunk := range chunks {
		rs := vote.RoundStats{
			Duration:           roundStats.Duration,
			RandUsed:           roundStats.RandUsed,
			ActiveVoteContexts: chunk,
		}

		hb := &packet.FPCHeartbeat{
			OwnID:      nodeID,
			RoundStats: rs,
		}

		finalizedMutex.Lock()
		hb.Finalized = finalized
		finalized = make(map[string]vote.Opinion)
		finalizedMutex.Unlock()

		data, err := packet.NewFPCHeartbeatMessage(hb)
		if err != nil {
			log.Info(err, " - FPC heartbeat message skipped")
			return
		}

		log.Info("Client: onRoundExecuted data size: ", len(data))

		if _, err = managedConn.Write(data); err != nil {
			log.Debugw("Error while writing to connection", "Description", err)
			return
		}
	}
}

func splitFPCVoteContext(ctx map[string]*vote.Context) (chunk []map[string]*vote.Context) {
	chunk = make([]map[string]*vote.Context, 1)
	i, counter := 0, 0
	chunk[i] = make(map[string]*vote.Context)

	if len(ctx) < maxVoteContext {
		chunk[i] = ctx
		return
	}

	for conflictID, voteCtx := range ctx {
		counter++
		if counter >= maxVoteContext {
			counter = 0
			i++
			chunk = append(chunk, make(map[string]*vote.Context))
		}
		chunk[i][conflictID] = voteCtx
	}

	return
}
