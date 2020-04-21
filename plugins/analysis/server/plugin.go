package server

import (
	"encoding/hex"
	"errors"
	"math"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/removenode"
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
		log.Infof("Starting Server (port %d) ... done", config.Node.GetInt(CFG_SERVER_PORT))
	}))
	server.Events.Shutdown.Attach(events.NewClosure(func() {
		log.Info("Stopping Server ... done")
	}))
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Analysis Server", func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting Server (port %d) ... done", config.Node.GetInt(CFG_SERVER_PORT))
		go server.Listen("0.0.0.0", config.Node.GetInt(CFG_SERVER_PORT))
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
	conn.SetTimeout(IDLE_TIMEOUT)

	var connectionState = STATE_INITIAL
	var receiveBuffer []byte
	var offset int
	var connectedNodeId string

	var onDisconnect *events.Closure

	onReceiveData := events.NewClosure(func(data []byte) {
		processIncomingPacket(&connectionState, &receiveBuffer, conn, data, &offset, &connectedNodeId)
	})
	onDisconnect = events.NewClosure(func() {
		Events.NodeOffline.Trigger(connectedNodeId)

		conn.Events.ReceiveData.Detach(onReceiveData)
		conn.Events.Close.Detach(onDisconnect)
	})

	conn.Events.ReceiveData.Attach(onReceiveData)
	conn.Events.Close.Attach(onDisconnect)

	maxPacketsSize := getMaxPacketSize(
		ping.MARSHALED_TOTAL_SIZE,
		addnode.MARSHALED_TOTAL_SIZE,
		removenode.MARSHALED_TOTAL_SIZE,
		connectnodes.MARSHALED_TOTAL_SIZE,
		disconnectnodes.MARSHALED_PACKET_HEADER,
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

func processIncomingPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int, connectedNodeId *string) {
	firstPackage := *connectionState == STATE_INITIAL

	if firstPackage || *connectionState == STATE_CONSECUTIVE {
		var err error
		if *connectionState, *receiveBuffer, err = parsePackageHeader(data); err != nil {
			Events.Error.Trigger(err)

			conn.Close()

			return
		}

		*offset = 0

		switch *connectionState {
		case STATE_ADD_NODE:
			*receiveBuffer = make([]byte, addnode.MARSHALED_TOTAL_SIZE)

		case STATE_PING:
			*receiveBuffer = make([]byte, ping.MARSHALED_TOTAL_SIZE)

		case STATE_CONNECT_NODES:
			*receiveBuffer = make([]byte, connectnodes.MARSHALED_TOTAL_SIZE)

		case STATE_DISCONNECT_NODES:
			*receiveBuffer = make([]byte, disconnectnodes.MARSHALED_TOTAL_SIZE)

		case STATE_REMOVE_NODE:
			*receiveBuffer = make([]byte, removenode.MARSHALED_TOTAL_SIZE)
		}
	}

	if firstPackage {
		if *connectionState != STATE_ADD_NODE {
			Events.Error.Trigger(ErrExpectedInitialAddNodePackage)
		} else {
			*connectionState = STATE_INITIAL_ADDNODE
		}
	}

	switch *connectionState {
	case STATE_INITIAL_ADDNODE:
		processIncomingAddNodePacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)

	case STATE_ADD_NODE:
		processIncomingAddNodePacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)

	case STATE_PING:
		processIncomingPingPacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)

	case STATE_CONNECT_NODES:
		processIncomingConnectNodesPacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)

	case STATE_DISCONNECT_NODES:
		processIncomingDisconnectNodesPacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)

	case STATE_REMOVE_NODE:
		processIncomingRemoveNodePacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)
	}
}

func parsePackageHeader(data []byte) (ConnectionState, []byte, error) {
	var connectionState ConnectionState
	var receiveBuffer []byte

	switch data[0] {
	case ping.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, ping.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_PING

	case addnode.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, addnode.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_ADD_NODE

	case connectnodes.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, connectnodes.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_CONNECT_NODES

	case disconnectnodes.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, disconnectnodes.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_DISCONNECT_NODES

	case removenode.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, removenode.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_REMOVE_NODE

	default:
		return 0, nil, ErrInvalidPackageHeader
	}

	return connectionState, receiveBuffer, nil
}

func processIncomingAddNodePacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int, connectedNodeId *string) {
	remainingCapacity := int(math.Min(float64(addnode.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < addnode.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if addNodePacket, err := addnode.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(err)

			conn.Close()

			return
		} else {
			nodeId := hex.EncodeToString(addNodePacket.NodeId)

			Events.AddNode.Trigger(nodeId)

			if *connectionState == STATE_INITIAL_ADDNODE {
				*connectedNodeId = nodeId

				Events.NodeOnline.Trigger(nodeId)
			}
		}

		*connectionState = STATE_CONSECUTIVE

		if *offset+len(data) > addnode.MARSHALED_TOTAL_SIZE {
			processIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset, connectedNodeId)
		}
	}
}

func processIncomingPingPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int, connectedNodeId *string) {
	remainingCapacity := int(math.Min(float64(ping.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < ping.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if _, err := ping.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(err)

			conn.Close()

			return
		}

		*connectionState = STATE_CONSECUTIVE

		if *offset+len(data) > ping.MARSHALED_TOTAL_SIZE {
			processIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset, connectedNodeId)
		}
	}
}

func processIncomingConnectNodesPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int, connectedNodeId *string) {
	remainingCapacity := int(math.Min(float64(connectnodes.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < connectnodes.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if connectNodesPacket, err := connectnodes.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(err)

			conn.Close()

			return
		} else {
			sourceNodeId := hex.EncodeToString(connectNodesPacket.SourceId)
			targetNodeId := hex.EncodeToString(connectNodesPacket.TargetId)

			Events.ConnectNodes.Trigger(sourceNodeId, targetNodeId)
		}

		*connectionState = STATE_CONSECUTIVE

		if *offset+len(data) > connectnodes.MARSHALED_TOTAL_SIZE {
			processIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset, connectedNodeId)
		}
	}
}

func processIncomingDisconnectNodesPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int, connectedNodeId *string) {
	remainingCapacity := int(math.Min(float64(disconnectnodes.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < disconnectnodes.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if disconnectNodesPacket, err := disconnectnodes.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(err)

			conn.Close()

			return
		} else {
			sourceNodeId := hex.EncodeToString(disconnectNodesPacket.SourceId)
			targetNodeId := hex.EncodeToString(disconnectNodesPacket.TargetId)

			Events.DisconnectNodes.Trigger(sourceNodeId, targetNodeId)
		}

		*connectionState = STATE_CONSECUTIVE

		if *offset+len(data) > disconnectnodes.MARSHALED_TOTAL_SIZE {
			processIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset, connectedNodeId)
		}
	}
}

func processIncomingRemoveNodePacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int, connectedNodeId *string) {
	remainingCapacity := int(math.Min(float64(removenode.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < removenode.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if removeNodePacket, err := removenode.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(err)

			conn.Close()

			return
		} else {
			nodeId := hex.EncodeToString(removeNodePacket.NodeId)

			Events.RemoveNode.Trigger(nodeId)

		}

		*connectionState = STATE_CONSECUTIVE

		if *offset+len(data) > addnode.MARSHALED_TOTAL_SIZE {
			processIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset, connectedNodeId)
		}
	}
}
