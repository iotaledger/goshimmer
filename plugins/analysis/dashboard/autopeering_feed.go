package dashboard

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	autopeeringWorkerCount     = 1
	autopeeringWorkerQueueSize = 500
	autopeeringWorkerPool      *workerpool.WorkerPool
)

// JSON encoded websocket message for adding a node
type addNode struct {
	ID string `json:"id"`
}

// JSON encoded websocket message for removing a node
type removeNode struct {
	ID string `json:"id"`
}

// JSON encoded websocket message for connecting two nodes
type connectNodes struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// JSON encoded websocket message for disconnecting two nodes
type disconnectNodes struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func configureAutopeeringWorkerPool() {
	// create a new worker pool for processing autopeering updates coming from analysis server
	autopeeringWorkerPool = workerpool.New(func(task workerpool.Task) {
		// determine what msg to send based on first parameter
		// first parameter is always a letter denoting what to do with the following string or strings
		x := fmt.Sprintf("%v", task.Param(0))
		switch x {
		case "A":
			sendAddNode(task.Param(1).(string))
		case "a":
			sendRemoveNode(task.Param(1).(string))
		case "C":
			sendConnectNodes(task.Param(1).(string), task.Param(2).(string))
		case "c":
			sendDisconnectNodes(task.Param(1).(string), task.Param(2).(string))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(autopeeringWorkerCount), workerpool.QueueSize(autopeeringWorkerQueueSize))
}

// send and addNode msg to all connected ws clients
func sendAddNode(nodeID string) {
	broadcastWsMessage(&wsmsg{MsgTypeAddNode, &addNode{nodeID}}, true)
}

// send a removeNode msg to all connected ws clients
func sendRemoveNode(nodeID string) {
	broadcastWsMessage(&wsmsg{MsgTypeRemoveNode, &removeNode{nodeID}}, true)
}

// send a connectNodes msg to all connected ws clients
func sendConnectNodes(source string, target string) {
	broadcastWsMessage(&wsmsg{MsgTypeConnectNodes, &connectNodes{
		Source: source,
		Target: target,
	}}, true)
}

// send disconnectNodes to all connected ws clients
func sendDisconnectNodes(source string, target string) {
	broadcastWsMessage(&wsmsg{MsgTypeDisconnectNodes, &disconnectNodes{
		Source: source,
		Target: target,
	}}, true)
}

// runs autopeering feed to propagate autopeering events from analysis server to frontend
func runAutopeeringFeed() {
	// closures for the different events
	notifyAddNode := events.NewClosure(func(nodeID string) {
		autopeeringWorkerPool.Submit("A", nodeID)
	})
	notifyRemoveNode := events.NewClosure(func(nodeID string) {
		autopeeringWorkerPool.Submit("a", nodeID)
	})
	notifyConnectNodes := events.NewClosure(func(source string, target string) {
		autopeeringWorkerPool.Submit("C", source, target)
	})
	notifyDisconnectNodes := events.NewClosure(func(source string, target string) {
		autopeeringWorkerPool.Submit("c", source, target)
	})

	if err := daemon.BackgroundWorker("Analysis-Dashboard[AutopeeringVisualizer]", func(shutdownSignal <-chan struct{}) {
		// connect closures (submitting tasks) to events of the analysis server
		analysisserver.Events.AddNode.Attach(notifyAddNode)
		defer analysisserver.Events.AddNode.Detach(notifyAddNode)
		analysisserver.Events.RemoveNode.Attach(notifyRemoveNode)
		defer analysisserver.Events.RemoveNode.Detach(notifyRemoveNode)
		analysisserver.Events.ConnectNodes.Attach(notifyConnectNodes)
		defer analysisserver.Events.ConnectNodes.Detach(notifyConnectNodes)
		analysisserver.Events.DisconnectNodes.Attach(notifyDisconnectNodes)
		defer analysisserver.Events.DisconnectNodes.Detach(notifyDisconnectNodes)
		autopeeringWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Analysis-Dashboard[AutopeeringVisualizer] ...")
		autopeeringWorkerPool.Stop()
		log.Info("Stopping Analysis-Dashboard[AutopeeringVisualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// creates event handlers for replaying autopeering events on them
func createAutopeeringEventHandlers(wsClient *websocket.Conn, nodeCallbackFactory func(*websocket.Conn, string) func(string), linkCallbackFactory func(*websocket.Conn, string) func(string, string)) *analysisserver.EventHandlers {
	return &analysisserver.EventHandlers{
		AddNode:         nodeCallbackFactory(wsClient, "A"),
		RemoveNode:      nodeCallbackFactory(wsClient, "a"),
		ConnectNodes:    linkCallbackFactory(wsClient, "C"),
		DisconnectNodes: linkCallbackFactory(wsClient, "c"),
	}
}

// creates callback function for addNode and removeNode events
func createSyncNodeCallback(ws *websocket.Conn, msgType string) func(nodeID string) {
	return func(nodeID string) {
		var wsMessage *wsmsg
		switch msgType {
		case "A":
			wsMessage = &wsmsg{MsgTypeAddNode, &addNode{nodeID}}
		case "a":
			wsMessage = &wsmsg{MsgTypeRemoveNode, &removeNode{nodeID}}
		}
		if err := ws.WriteJSON(wsMessage); err != nil {
			return
		}
		if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
			return
		}
	}
}

// creates callback function for connectNodes and disconnectNodes events
func createSyncLinkCallback(ws *websocket.Conn, msgType string) func(sourceID string, targetID string) {
	return func(sourceID string, targetID string) {
		var wsMessage *wsmsg
		switch msgType {
		case "C":
			wsMessage = &wsmsg{MsgTypeConnectNodes, &connectNodes{sourceID, targetID}}
		case "c":
			wsMessage = &wsmsg{MsgTypeDisconnectNodes, &disconnectNodes{sourceID, targetID}}
		}
		if err := ws.WriteJSON(wsMessage); err != nil {
			return
		}
		if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
			return
		}
	}
}
