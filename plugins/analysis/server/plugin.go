package server

import (
	"encoding/hex"
	"math"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/tcp"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/removenode"
	"github.com/pkg/errors"
)

var server *tcp.Server

func Configure(plugin *node.Plugin) {
	server = tcp.NewServer()

	server.Events.Connect.Attach(events.NewClosure(HandleConnection))
	server.Events.Error.Attach(events.NewClosure(func(err error) {
		plugin.LogFailure("error in server: " + err.Error())
	}))
	server.Events.Start.Attach(events.NewClosure(func() {
		plugin.LogSuccess("Starting Server (port " + strconv.Itoa(*SERVER_PORT.Value) + ") ... done")
	}))
	server.Events.Shutdown.Attach(events.NewClosure(func() {
		plugin.LogSuccess("Stopping Server ... done")
	}))
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Analysis Server", func() {
		plugin.LogInfo("Starting Server (port " + strconv.Itoa(*SERVER_PORT.Value) + ") ...")

		server.Listen(*SERVER_PORT.Value)
	})
}

func Shutdown(plugin *node.Plugin) {
	plugin.LogInfo("Stopping Server ...")

	server.Shutdown()
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
			Events.Error.Trigger(errors.New("expected initial add node package"))
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
		processIncomingAddNodePacket(connectionState, receiveBuffer, conn, data, offset, connectedNodeId)
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
		return 0, nil, errors.New("invalid package header")
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
