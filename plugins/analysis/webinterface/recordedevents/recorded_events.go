package recordedevents

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/analysis/server"
    "github.com/iotaledger/goshimmer/plugins/analysis/webinterface/types"
    "time"
)

var recordedEvents = make([]types.EventHandlersConsumer, 0)

var nodes = make(map[string]bool)
var links = make(map[string]map[string]bool)

func Configure(plugin *node.Plugin) {
    server.Events.AddNode.Attach(func(nodeId string) {
        nodes[nodeId] = false
        /*
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.AddNode(nodeId)
        })
        */
    })

    server.Events.RemoveNode.Attach(func(nodeId string) {
        delete(nodes, nodeId)
        /*
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.AddNode(nodeId)
        })
        */
    })

    server.Events.NodeOnline.Attach(func(nodeId string) {
        nodes[nodeId] = true
        /*
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.NodeOnline(nodeId)
        })
        */
    })

    server.Events.NodeOffline.Attach(func(nodeId string) {
        nodes[nodeId] = false
        /*
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.NodeOffline(nodeId)
        })
        */
    })

    server.Events.ConnectNodes.Attach(func(sourceId string, targetId string) {
        connectionMap, connectionMapExists := links[sourceId]
        if !connectionMapExists {
            connectionMap = make(map[string]bool)

            links[sourceId] = connectionMap
        }
        connectionMap[targetId] = true
        /*
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.ConnectNodes(sourceId, targetId)
        })
        */
    })

    server.Events.DisconnectNodes.Attach(func(sourceId string, targetId string) {
        connectionMap, connectionMapExists := links[sourceId]
        if connectionMapExists {
            delete(connectionMap, targetId)
        }
        /*
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.ConnectNodes(sourceId, targetId)
        })
        */
    })
}

func Replay(handlers *types.EventHandlers, delay time.Duration) {
    for nodeId, online := range nodes {
        handlers.AddNode(nodeId)
        if online {
            handlers.NodeOnline(nodeId)
        } else {
            handlers.NodeOffline(nodeId)
        }
    }

    for sourceId, targetMap := range links {
        for targetId, _ := range targetMap {
            handlers.ConnectNodes(sourceId, targetId)
        }
    }
    /*
    for _, recordedEvent := range recordedEvents {
        recordedEvent(handlers)

        if delay != time.Duration(0) {
            time.Sleep(delay)
        }
    }
    */
}
