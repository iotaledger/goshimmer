package dashboard

import (
	"context"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
)

var (
	autoPeeringWorkerCount     = 1
	autoPeeringWorkerQueueSize = 500
	autoPeeringWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

// JSON encoded websocket message for adding a node
type addNode struct {
	NetworkVersion string `json:"networkVersion"`
	ID             string `json:"id"`
}

// JSON encoded websocket message for removing a node
type removeNode struct {
	NetworkVersion string `json:"networkVersion"`
	ID             string `json:"id"`
}

// JSON encoded websocket message for connecting two nodes
type connectNodes struct {
	NetworkVersion string `json:"networkVersion"`
	Source         string `json:"source"`
	Target         string `json:"target"`
}

// JSON encoded websocket message for disconnecting two nodes
type disconnectNodes struct {
	NetworkVersion string `json:"networkVersion"`
	Source         string `json:"source"`
	Target         string `json:"target"`
}

func configureAutopeeringWorkerPool() {
	// create a new worker pool for processing autopeering updates coming from analysis server
	autoPeeringWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		// determine what msg to send based on first parameter
		// first parameter is always a letter denoting what to do with the following string or strings
		x := fmt.Sprintf("%v", task.Param(0))
		switch x {
		case "A":
			sendAddNode(task.Param(1).(*analysisserver.AddNodeEvent))
		case "a":
			sendRemoveNode(task.Param(1).(*analysisserver.RemoveNodeEvent))
		case "C":
			sendConnectNodes(task.Param(1).(*analysisserver.ConnectNodesEvent))
		case "c":
			sendDisconnectNodes(task.Param(1).(*analysisserver.DisconnectNodesEvent))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(autoPeeringWorkerCount), workerpool.QueueSize(autoPeeringWorkerQueueSize))
}

// send and addNode msg to all connected ws clients
func sendAddNode(eventStruct *analysisserver.AddNodeEvent) {
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeAddNode,
		Data: &addNode{
			NetworkVersion: eventStruct.NetworkVersion,
			ID:             eventStruct.NodeID,
		},
	}, true)
}

// send a removeNode msg to all connected ws clients
func sendRemoveNode(eventStruct *analysisserver.RemoveNodeEvent) {
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeRemoveNode,
		Data: &removeNode{
			NetworkVersion: eventStruct.NetworkVersion,
			ID:             eventStruct.NodeID,
		},
	}, true)
}

// send a connectNodes msg to all connected ws clients
func sendConnectNodes(eventStruct *analysisserver.ConnectNodesEvent) {
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeConnectNodes,
		Data: &connectNodes{
			NetworkVersion: eventStruct.NetworkVersion,
			Source:         eventStruct.SourceID,
			Target:         eventStruct.TargetID,
		},
	}, true)
}

// send disconnectNodes to all connected ws clients
func sendDisconnectNodes(eventStruct *analysisserver.DisconnectNodesEvent) {
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeDisconnectNodes,
		Data: &disconnectNodes{
			NetworkVersion: eventStruct.NetworkVersion,
			Source:         eventStruct.SourceID,
			Target:         eventStruct.TargetID,
		},
	}, true)
}

// runs autopeering feed to propagate autopeering events from analysis server to frontend
func runAutopeeringFeed() {
	// closures for the different events
	notifyAddNode := events.NewClosure(func(eventStruct *analysisserver.AddNodeEvent) {
		autoPeeringWorkerPool.Submit("A", eventStruct)
	})
	notifyRemoveNode := events.NewClosure(func(eventStruct *analysisserver.RemoveNodeEvent) {
		autoPeeringWorkerPool.Submit("a", eventStruct)
	})
	notifyConnectNodes := events.NewClosure(func(eventStruct *analysisserver.ConnectNodesEvent) {
		autoPeeringWorkerPool.Submit("C", eventStruct)
	})
	notifyDisconnectNodes := events.NewClosure(func(eventStruct *analysisserver.DisconnectNodesEvent) {
		autoPeeringWorkerPool.Submit("c", eventStruct)
	})

	if err := daemon.BackgroundWorker("AnalysisDashboard[AutopeeringVisualizer]", func(ctx context.Context) {
		// connect closures (submitting tasks) to events of the analysis server
		analysisserver.Events.AddNode.Attach(notifyAddNode)
		defer analysisserver.Events.AddNode.Detach(notifyAddNode)
		analysisserver.Events.RemoveNode.Attach(notifyRemoveNode)
		defer analysisserver.Events.RemoveNode.Detach(notifyRemoveNode)
		analysisserver.Events.ConnectNodes.Attach(notifyConnectNodes)
		defer analysisserver.Events.ConnectNodes.Detach(notifyConnectNodes)
		analysisserver.Events.DisconnectNodes.Attach(notifyDisconnectNodes)
		defer analysisserver.Events.DisconnectNodes.Detach(notifyDisconnectNodes)
		<-ctx.Done()
		log.Info("Stopping AnalysisDashboard[AutopeeringVisualizer] ...")
		autoPeeringWorkerPool.Stop()
		log.Info("Stopping AnalysisDashboard[AutopeeringVisualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// creates event handlers for replaying autopeering events on them
func createAutopeeringEventHandlers(wsClient *websocket.Conn) *analysisserver.EventHandlers {
	return &analysisserver.EventHandlers{
		AddNode:         createAddNodeCallback(wsClient),
		RemoveNode:      createRemoveNodeCallback(wsClient),
		ConnectNodes:    createConnectNodesCallback(wsClient),
		DisconnectNodes: createDisconnectNodesCallback(wsClient),
	}
}

// creates callback function for addNode  event
func createAddNodeCallback(ws *websocket.Conn) func(event *analysisserver.AddNodeEvent) {
	return func(event *analysisserver.AddNodeEvent) {
		wsMessage := &wsmsg{
			Type: MsgTypeAddNode,
			Data: &addNode{
				NetworkVersion: event.NetworkVersion,
				ID:             event.NodeID,
			},
		}
		if err := sendJSON(ws, wsMessage); err != nil {
			log.Error(err.Error())
		}
	}
}

// creates callback function for removeNode  event
func createRemoveNodeCallback(ws *websocket.Conn) func(event *analysisserver.RemoveNodeEvent) {
	return func(event *analysisserver.RemoveNodeEvent) {
		wsMessage := &wsmsg{
			Type: MsgTypeRemoveNode,
			Data: &removeNode{
				NetworkVersion: event.NetworkVersion,
				ID:             event.NodeID,
			},
		}
		if err := sendJSON(ws, wsMessage); err != nil {
			log.Error(err.Error())
		}
	}
}

// creates callback function for connectNodes  event
func createConnectNodesCallback(ws *websocket.Conn) func(event *analysisserver.ConnectNodesEvent) {
	return func(event *analysisserver.ConnectNodesEvent) {
		wsMessage := &wsmsg{
			Type: MsgTypeConnectNodes,
			Data: &connectNodes{
				NetworkVersion: event.NetworkVersion,
				Source:         event.SourceID,
				Target:         event.TargetID,
			},
		}
		if err := sendJSON(ws, wsMessage); err != nil {
			log.Error(err.Error())
		}
	}
}

// creates callback function for disconnectNodes  event
func createDisconnectNodesCallback(ws *websocket.Conn) func(event *analysisserver.DisconnectNodesEvent) {
	return func(event *analysisserver.DisconnectNodesEvent) {
		wsMessage := &wsmsg{
			Type: MsgTypeDisconnectNodes,
			Data: &disconnectNodes{
				NetworkVersion: event.NetworkVersion,
				Source:         event.SourceID,
				Target:         event.TargetID,
			},
		}
		if err := sendJSON(ws, wsMessage); err != nil {
			log.Error(err.Error())
		}
	}
}
