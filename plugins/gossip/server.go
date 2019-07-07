package gossip

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/tcp"
	"github.com/iotaledger/goshimmer/packages/node"
)

var TCPServer = tcp.NewServer()

func configureServer(plugin *node.Plugin) {
	TCPServer.Events.Connect.Attach(events.NewClosure(func(conn *network.ManagedConnection) {
		protocol := newProtocol(conn)

		// print protocol errors
		protocol.Events.Error.Attach(events.NewClosure(func(err errors.IdentifiableError) {
			plugin.LogFailure(err.Error())
		}))

		// store protocol in neighbor if its a neighbor calling
		protocol.Events.ReceiveIdentification.Attach(events.NewClosure(func(identity *identity.Identity) {
			if protocol.Neighbor != nil {
				protocol.Neighbor.acceptedProtocolMutex.Lock()
				if protocol.Neighbor.AcceptedProtocol == nil {
					protocol.Neighbor.AcceptedProtocol = protocol

					protocol.Conn.Events.Close.Attach(events.NewClosure(func() {
						protocol.Neighbor.acceptedProtocolMutex.Lock()
						defer protocol.Neighbor.acceptedProtocolMutex.Unlock()

						protocol.Neighbor.AcceptedProtocol = nil
					}))
				}
				protocol.Neighbor.acceptedProtocolMutex.Unlock()
			}
		}))

		// drop the "secondary" connection upon successful handshake
		protocol.Events.HandshakeCompleted.Attach(events.NewClosure(func() {
			if protocol.Neighbor.Identity.StringIdentifier <= accountability.OwnId().StringIdentifier {
				protocol.Neighbor.initiatedProtocolMutex.Lock()
				var initiatedProtocolConn *network.ManagedConnection
				if protocol.Neighbor.InitiatedProtocol != nil {
					initiatedProtocolConn = protocol.Neighbor.InitiatedProtocol.Conn
				}
				protocol.Neighbor.initiatedProtocolMutex.Unlock()

				if initiatedProtocolConn != nil {
					_ = initiatedProtocolConn.Close()
				}
			}

			protocol.Neighbor.Events.ProtocolConnectionEstablished.Trigger(protocol)
		}))

		go protocol.Init()
	}))

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		plugin.LogInfo("Stopping TCP Server ...")

		TCPServer.Shutdown()
	}))
}

func runServer(plugin *node.Plugin) {
	plugin.LogInfo("Starting TCP Server (port " + strconv.Itoa(*PORT.Value) + ") ...")

	daemon.BackgroundWorker("Gossip TCP Server", func() {
		plugin.LogSuccess("Starting TCP Server (port " + strconv.Itoa(*PORT.Value) + ") ... done")

		TCPServer.Listen(*PORT.Value)

		plugin.LogSuccess("Stopping TCP Server ... done")
	})
}
