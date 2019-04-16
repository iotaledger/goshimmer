package recordedevents

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/analysis/server"
    "github.com/iotaledger/goshimmer/plugins/analysis/webinterface/types"
    "time"
)

var recordedEvents = make([]types.EventHandlersConsumer, 0)

func Configure(plugin *node.Plugin) {
    server.Events.AddNode.Attach(func(nodeId string) {
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.AddNode(nodeId)
        })
    })

    server.Events.NodeOnline.Attach(func(nodeId string) {
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.NodeOnline(nodeId)
        })
    })

    server.Events.NodeOffline.Attach(func(nodeId string) {
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.NodeOffline(nodeId)
        })
    })

    server.Events.ConnectNodes.Attach(func(sourceId string, targetId string) {
        recordedEvents = append(recordedEvents, func(handlers *types.EventHandlers) {
            handlers.ConnectNodes(sourceId, targetId)
        })
    })
}

func Replay(handlers *types.EventHandlers, delay time.Duration) {
    for _, recordedEvent := range recordedEvents {
        recordedEvent(handlers)

        if delay != time.Duration(0) {
            time.Sleep(delay)
        }
    }
}
