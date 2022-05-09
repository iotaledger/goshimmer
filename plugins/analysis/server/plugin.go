package server

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/protocol"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
)

const (
	// PluginName is the name of the analysis server plugin.
	PluginName = "AnalysisServer"

	// CfgAnalysisServerBindAddress defines the bind address of the analysis server.
	CfgAnalysisServerBindAddress = "analysis.server.bindAddress"

	// IdleTimeout defines the idle timeout of the read from the client's connection.
	IdleTimeout = 1 * time.Minute
)

type dependencies struct {
	dig.In

	Config *configuration.Configuration
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
	flag.String(CfgAnalysisServerBindAddress, "0.0.0.0:16178", "the bind address of the analysis server")
}

var (
	// Plugin is the plugin instance of the analysis server plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	server *tcp.TCPServer
	prot   *protocol.Protocol
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	server = tcp.NewServer()

	server.Events.Connect.Hook(event.NewClosure(func(event *tcp.ConnectEvent) { HandleConnection(event.ManagedConnection) }))
	server.Events.Error.Hook(event.NewClosure(func(err error) {
		log.Errorf("error in server: %s", err.Error())
	}))
	Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("error in analysis server: %s", err.Error())
	}))
}

func run(_ *node.Plugin) {
	bindAddr := deps.Config.String(CfgAnalysisServerBindAddress)
	addr, portStr, err := net.SplitHostPort(bindAddr)
	if err != nil {
		log.Fatal("invalid bind address in %s: %s", CfgAnalysisServerBindAddress, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatal("invalid port in %s: %s", CfgAnalysisServerBindAddress, err)
	}

	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
		defer log.Infof("Stopping %s ... done", PluginName)

		// connect protocol events to processors
		prot = protocol.New(packet.AnalysisMsgRegistry())
		wireUp(prot)

		go server.Listen(addr, port)

		<-ctx.Done()
		log.Info("Stopping Server ...")
		server.Shutdown()
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
	runEventsRecordManager()
}

// HandleConnection handles the given connection.
func HandleConnection(conn *network.ManagedConnection) {
	if err := conn.SetReadTimeout(IdleTimeout); err != nil {
		log.Warnw("Error setting read timeout; closing connection", "err", err)
		_ = conn.Close()
		return
	}
	onReceiveData := event.NewClosure(func(event *network.ReceivedDataEvent) {
		if _, err := prot.Read(event.Data); err != nil {
			log.Debugw("Invalid message received; closing connection", "err", err)
			_ = conn.Close()
		}
	})
	conn.Events.ReceiveData.Hook(onReceiveData)
	// starts the protocol and reads from its connection
	go func() {
		buffer := make([]byte, 2048)
		_, err := conn.Read(buffer)
		if err != nil && err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Warnw("Read error", "err", err)
		}
		// always close the connection when we've stopped reading from it
		_ = conn.Close()
	}()
}

// wireUp connects the Received events of the protocol to the packet specific processor.
func wireUp(p *protocol.Protocol) {
	p.Events.Received[packet.MessageTypeHeartbeat].Hook(event.NewClosure(func(data []byte) {
		processHeartbeatPacket(data)
	}))
	p.Events.Received[packet.MessageTypeMetricHeartbeat].Hook(event.NewClosure(func(data []byte) {
		processMetricHeartbeatPacket(data)
	}))
}

// processHeartbeatPacket parses the serialized data into a Heartbeat packet and triggers its event.
func processHeartbeatPacket(data []byte) {
	heartbeatPacket, err := packet.ParseHeartbeat(data)
	if err != nil {
		if !errors.Is(err, packet.ErrInvalidHeartbeatNetworkVersion) {
			Events.Error.Trigger(err)
		}
		return
	}
	updateAutopeeringMap(heartbeatPacket)
}

// processMetricHeartbeatPacket parses the serialized data into a Metric Heartbeat packet and triggers its event.
// Note that the ParseMetricHeartbeat function will return an error if the hb version field is different than banner.SimplifiedAppVersion,
// thus the hb will be discarded.
func processMetricHeartbeatPacket(data []byte) {
	hb, err := packet.ParseMetricHeartbeat(data)
	if err != nil {
		if !errors.Is(err, packet.ErrInvalidMetricHeartbeatVersion) {
			Events.Error.Trigger(err)
		}
		return
	}
	Events.MetricHeartbeat.Trigger(hb)
}
