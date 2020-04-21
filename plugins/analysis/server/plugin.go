package server

import (
	"errors"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var (
	ErrInvalidPackageHeader          = errors.New("invalid package header")
	ErrExpectedInitialAddNodePackage = errors.New("expected initial add node package")
	server                           *tcp.TCPServer
	log                              *logger.Logger
)

func Configure(plugin *node.Plugin) {
	log = logger.NewLogger("Analysis-Server")
	server = tcp.NewServer()

	server.Events.Connect.Attach(events.NewClosure(HandleConnection))
	server.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("error in server: %s", err.Error())
	}))
	server.Events.Start.Attach(events.NewClosure(func() {
		log.Infof("Starting Server (port %d) ... done", config.Node.GetInt(CfgServerPort))
	}))
	server.Events.Shutdown.Attach(events.NewClosure(func() {
		log.Info("Stopping Server ... done")
	}))
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Analysis Server", func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting Server (port %d) ... done", config.Node.GetInt(CfgServerPort))
		go server.Listen("0.0.0.0", config.Node.GetInt(CfgServerPort))
		<-shutdownSignal
		Shutdown()
	}, shutdown.PriorityAnalysis)
}

func Shutdown() {
	log.Info("Stopping Server ...")
	server.Shutdown()
	log.Info("Stopping Server ... done")
}

func HandleConnection(conn *network.ManagedConnection) {
	err := conn.SetTimeout(IdleTimeout)
	if err!=nil {
		log.Errorf(err.Error())
	}

	var connectionState byte
	var receiveBuffer []byte

	var onDisconnect *events.Closure

	onReceiveData := events.NewClosure(func(data []byte) {
		processIncomingPacket(&connectionState, &receiveBuffer, conn, data)
	})
	onDisconnect = events.NewClosure(func() {
		conn.Events.ReceiveData.Detach(onReceiveData)
		conn.Events.Close.Detach(onDisconnect)
	})

	conn.Events.ReceiveData.Attach(onReceiveData)
	conn.Events.Close.Attach(onDisconnect)

	maxPacketsSize := getMaxPacketSize(
		heartbeat.MaxMarshaledTotalSize,
	)

	go conn.Read(make([]byte, maxPacketsSize))
}

func getMaxPacketSize(packetSizes ...int) int {
	maxPacketSize := 0

	for _, packetSize := range packetSizes {
		if packetSize > maxPacketSize {
			maxPacketSize = packetSize
		}
	}

	return maxPacketSize
}

func processIncomingPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte) {
	var err error
	if *connectionState, *receiveBuffer, err = parsePackageHeader(data); err != nil {
		Events.Error.Trigger(err)

		conn.Close()

		return
	}

	switch *connectionState {
	case StateHeartbeat:
		processHeartbeatPacket(connectionState, receiveBuffer, conn, data)
	}

}

func parsePackageHeader(data []byte) (ConnectionState, []byte, error) {
	var connectionState ConnectionState
	var receiveBuffer []byte

	switch data[0] {
	case heartbeat.MarshaledPacketHeader:
		receiveBuffer = make([]byte, heartbeat.MaxMarshaledTotalSize)

		connectionState = StateHeartbeat

	default:
		return 0, nil, ErrInvalidPackageHeader
	}

	return connectionState, receiveBuffer, nil
}

func processHeartbeatPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte) {

	heartbeatPacket, err := heartbeat.Unmarshal(data)
	if err != nil {
		Events.Error.Trigger(err)
		conn.Close()
		return
	}
	Events.Heartbeat.Trigger(*heartbeatPacket)
}
