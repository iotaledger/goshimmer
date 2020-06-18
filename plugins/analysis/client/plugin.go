package client

import (
	"net"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
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
	// maxVoteContext defines the maximum number of vote context to fit into an FPC update
	maxVoteContext = 50
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}

var (
	// Plugin is the plugin instance of the analysis client plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, run)
	log    *logger.Logger
	conn   = &connector{}
)

func run(_ *node.Plugin) {
	finalized = make(map[string]vote.Opinion)
	log = logger.NewLogger(PluginName)

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

type connector struct {
	mu sync.Mutex

	conn *network.ManagedConnection

	closeOnce sync.Once
	closing   chan struct{}
}

func (c *connector) Start() {
	c.closing = make(chan struct{})
	c.new()
}

func (c *connector) Stop() {
	c.closeOnce.Do(func() {
		close(c.closing)
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

func (c *connector) new() {
	select {
	case _ = <-c.closing:
		return
	default:
		c.mu.Lock()
		defer c.mu.Unlock()

		conn, err := net.Dial("tcp", config.Node.GetString(CfgServerAddress))
		if err != nil {
			time.AfterFunc(1*time.Minute, c.new)
			log.Warn(err)
			return
		}
		c.conn = network.NewManagedConnection(conn)
		c.conn.Events.Close.Attach(events.NewClosure(c.new))
	}
}

func (c *connector) Write(b []byte) (int, error) {
	// TODO: check that start was called
	// TODO: check that Stop was not called
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Write(b)
}
