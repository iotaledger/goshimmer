package webinterface

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/hive.go/events"
	"golang.org/x/net/websocket"
)

func dataStream(ws *websocket.Conn) {
	// create a wrapper for the websocket
	wsChan := NewWebSocketChannel(ws)
	defer wsChan.Close()

	// variables and factory methods for the async calls after the initial replay
	var replayMutex sync.RWMutex
	createAsyncNodeCallback := func(wsChan *WebSocketChannel, messagePrefix string) func(string) {
		return func(nodeId string) {
			go func() {
				replayMutex.RLock()
				defer replayMutex.RUnlock()

				wsChan.Write(messagePrefix + nodeId)
			}()
		}
	}
	createAsyncLinkCallback := func(wsChan *WebSocketChannel, messagePrefix string) func(string, string) {
		return func(sourceId string, targetId string) {
			go func() {
				replayMutex.RLock()
				defer replayMutex.RUnlock()

				wsChan.Write(messagePrefix + sourceId + targetId)
			}()
		}
	}

	// wait with firing the callbacks until the replay is complete
	replayMutex.Lock()

	// create and register the dynamic callbacks
	addNodeClosure := events.NewClosure(createAsyncNodeCallback(wsChan, "A"))
	removeNodeClosure := events.NewClosure(createAsyncNodeCallback(wsChan, "a"))
	connectNodesClosure := events.NewClosure(createAsyncLinkCallback(wsChan, "C"))
	disconnectNodesClosure := events.NewClosure(createAsyncLinkCallback(wsChan, "c"))
	server.Events.AddNode.Attach(addNodeClosure)
	server.Events.RemoveNode.Attach(removeNodeClosure)
	server.Events.ConnectNodes.Attach(connectNodesClosure)
	server.Events.DisconnectNodes.Attach(disconnectNodesClosure)

	// replay old events
	replayEvents(createEventHandlers(wsChan, createSyncNodeCallback, createSyncLinkCallback))

	// mark replay as complete
	replayMutex.Unlock()

	// wait until the connection breaks and keep it alive
	wsChan.KeepAlive()

	// unregister the callbacks
	server.Events.AddNode.Detach(addNodeClosure)
	server.Events.RemoveNode.Detach(removeNodeClosure)
	server.Events.ConnectNodes.Detach(connectNodesClosure)
	server.Events.DisconnectNodes.Detach(disconnectNodesClosure)
}

func createEventHandlers(wsChan *WebSocketChannel, nodeCallbackFactory func(*WebSocketChannel, string) func(string), linkCallbackFactory func(*WebSocketChannel, string) func(string, string)) *EventHandlers {
	return &EventHandlers{
		AddNode:         nodeCallbackFactory(wsChan, "A"),
		RemoveNode:      nodeCallbackFactory(wsChan, "a"),
		ConnectNodes:    linkCallbackFactory(wsChan, "C"),
		DisconnectNodes: linkCallbackFactory(wsChan, "c"),
	}
}

func createSyncNodeCallback(wsChan *WebSocketChannel, messagePrefix string) func(nodeId string) {
	return func(nodeId string) {
		wsChan.Write(messagePrefix + nodeId)
	}
}

func createSyncLinkCallback(wsChan *WebSocketChannel, messagePrefix string) func(sourceId string, targetId string) {
	return func(sourceId string, targetId string) {
		wsChan.Write(messagePrefix + sourceId + targetId)
	}
}
