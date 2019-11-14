package httpserver

import (
	"fmt"

	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/recordedevents"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/types"
	"github.com/iotaledger/hive.go/events"
	"golang.org/x/net/websocket"
)

func dataStream(ws *websocket.Conn) {
	func() {
		eventHandlers := &types.EventHandlers{
			AddNode:         func(nodeId string) { fmt.Fprint(ws, "A"+nodeId) },
			RemoveNode:      func(nodeId string) { fmt.Fprint(ws, "a"+nodeId) },
			ConnectNodes:    func(sourceId string, targetId string) { fmt.Fprint(ws, "C"+sourceId+targetId) },
			DisconnectNodes: func(sourceId string, targetId string) { fmt.Fprint(ws, "c"+sourceId+targetId) },
			NodeOnline:      func(nodeId string) { fmt.Fprint(ws, "O"+nodeId) },
			NodeOffline:     func(nodeId string) { fmt.Fprint(ws, "o"+nodeId) },
		}

		addNodeClosure := events.NewClosure(eventHandlers.AddNode)
		removeNodeClosure := events.NewClosure(eventHandlers.RemoveNode)
		connectNodesClosure := events.NewClosure(eventHandlers.ConnectNodes)
		disconnectNodesClosure := events.NewClosure(eventHandlers.DisconnectNodes)
		nodeOnlineClosure := events.NewClosure(eventHandlers.NodeOnline)
		nodeOfflineClosure := events.NewClosure(eventHandlers.NodeOffline)

		server.Events.AddNode.Attach(addNodeClosure)
		server.Events.RemoveNode.Attach(removeNodeClosure)
		server.Events.ConnectNodes.Attach(connectNodesClosure)
		server.Events.DisconnectNodes.Attach(disconnectNodesClosure)
		server.Events.NodeOnline.Attach(nodeOnlineClosure)
		server.Events.NodeOffline.Attach(nodeOfflineClosure)

		go recordedevents.Replay(eventHandlers)

		buf := make([]byte, 1)
	readFromWebsocket:
		for {
			if _, err := ws.Read(buf); err != nil {
				break readFromWebsocket
			}

			fmt.Fprint(ws, "_")
		}

		server.Events.AddNode.Detach(addNodeClosure)
		server.Events.RemoveNode.Detach(removeNodeClosure)
		server.Events.ConnectNodes.Detach(connectNodesClosure)
		server.Events.DisconnectNodes.Detach(disconnectNodesClosure)
		server.Events.NodeOnline.Detach(nodeOnlineClosure)
		server.Events.NodeOffline.Detach(nodeOfflineClosure)
	}()
}
