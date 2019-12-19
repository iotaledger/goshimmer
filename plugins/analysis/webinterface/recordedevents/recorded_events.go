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
		lock.Lock()
		defer lock.Unlock()

		if _, exists := nodes[nodeId]; !exists {
			nodes[nodeId] = false
		}
	}))

	server.Events.RemoveNode.Attach(events.NewClosure(func(nodeId string) {
		lock.Lock()
		defer lock.Unlock()

		delete(nodes, nodeId)
	}))

	server.Events.NodeOnline.Attach(events.NewClosure(func(nodeId string) {
		lock.Lock()
		defer lock.Unlock()

		nodes[nodeId] = true
	}))

	server.Events.NodeOffline.Attach(events.NewClosure(func(nodeId string) {
		lock.Lock()
		defer lock.Unlock()

		nodes[nodeId] = false
	}))

	server.Events.ConnectNodes.Attach(events.NewClosure(func(sourceId string, targetId string) {
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
		lock.Lock()
		defer lock.Unlock()

		connectionMap, connectionMapExists := links[sourceId]
		if connectionMapExists {
			delete(connectionMap, targetId)
		}
	}))
}

func Replay(handlers *types.EventHandlers) {
	for nodeId, online := range nodes {
		handlers.AddNode(nodeId)
		if online {
			handlers.NodeOnline(nodeId)
		} else {
			handlers.NodeOffline(nodeId)
		}
	}

	for sourceId, targetMap := range links {
		for targetId := range targetMap {
			handlers.ConnectNodes(sourceId, targetId)
		}
	}
}
