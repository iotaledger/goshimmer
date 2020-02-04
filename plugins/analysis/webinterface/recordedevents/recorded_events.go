package recordedevents

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/types"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

var nodes = make(map[string]bool)
var links = make(map[string]map[string]bool)

var lock sync.Mutex

func Configure(plugin *node.Plugin) {
	server.Events.AddNode.Attach(events.NewClosure(func(nodeId string) {
		plugin.Node.Logger.Debugw("AddNode", "nodeID", nodeId)
		lock.Lock()
		defer lock.Unlock()

		if _, exists := nodes[nodeId]; !exists {
			nodes[nodeId] = false
		}
	}))

	server.Events.RemoveNode.Attach(events.NewClosure(func(nodeId string) {
		plugin.Node.Logger.Debugw("RemoveNode", "nodeID", nodeId)
		lock.Lock()
		defer lock.Unlock()

		delete(nodes, nodeId)
	}))

	server.Events.NodeOnline.Attach(events.NewClosure(func(nodeId string) {
		plugin.Node.Logger.Debugw("NodeOnline", "nodeID", nodeId)
		lock.Lock()
		defer lock.Unlock()

		nodes[nodeId] = true
	}))

	server.Events.NodeOffline.Attach(events.NewClosure(func(nodeId string) {
		plugin.Node.Logger.Debugw("NodeOffline", "nodeID", nodeId)
		lock.Lock()
		defer lock.Unlock()

		nodes[nodeId] = false
	}))

	server.Events.ConnectNodes.Attach(events.NewClosure(func(sourceId string, targetId string) {
		plugin.Node.Logger.Debugw("ConnectNodes", "sourceID", sourceId, "targetId", targetId)
		lock.Lock()
		defer lock.Unlock()

		connectionMap, connectionMapExists := links[sourceId]
		if !connectionMapExists {
			connectionMap = make(map[string]bool)

			links[sourceId] = connectionMap
		}
		connectionMap[targetId] = true
	}))

	server.Events.DisconnectNodes.Attach(events.NewClosure(func(sourceId string, targetId string) {
		plugin.Node.Logger.Debugw("DisconnectNodes", "sourceID", sourceId, "targetId", targetId)
		lock.Lock()
		defer lock.Unlock()

		connectionMap, connectionMapExists := links[sourceId]
		if connectionMapExists {
			delete(connectionMap, targetId)
		}
	}))
}

func getEventsToReplay() (map[string]bool, map[string]map[string]bool) {
	lock.Lock()
	defer lock.Unlock()

	copiedNodes := make(map[string]bool)
	for nodeId, online := range nodes {
		copiedNodes[nodeId] = online
	}

	copiedLinks := make(map[string]map[string]bool)
	for sourceId, targetMap := range links {
		copiedLinks[sourceId] = make(map[string]bool)
		for targetId := range targetMap {
			copiedLinks[sourceId][targetId] = true
		}
	}

	return copiedNodes, copiedLinks
}

func Replay(handlers *types.EventHandlers) {
	copiedNodes, copiedLinks := getEventsToReplay()

	for nodeId, online := range copiedNodes {
		handlers.AddNode(nodeId)
		if online {
			handlers.NodeOnline(nodeId)
		} else {
			handlers.NodeOffline(nodeId)
		}
	}

	for sourceId, targetMap := range copiedLinks {
		for targetId := range targetMap {
			handlers.ConnectNodes(sourceId, targetId)
		}
	}
}
