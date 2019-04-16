package httpserver

import (
    "fmt"
    "github.com/iotaledger/goshimmer/plugins/analysis/server"
    "github.com/iotaledger/goshimmer/plugins/analysis/webinterface/recordedevents"
    "github.com/iotaledger/goshimmer/plugins/analysis/webinterface/types"
    "golang.org/x/net/websocket"
)

func dataStream(ws *websocket.Conn) {
    eventHandlers := &types.EventHandlers{
        AddNode:         func(nodeId string) { fmt.Fprint(ws, "A"+nodeId) },
        RemoveNode:      func(nodeId string) { fmt.Fprint(ws, "a"+nodeId) },
        ConnectNodes:    func(sourceId string, targetId string) { fmt.Fprint(ws, "C"+sourceId+targetId) },
        DisconnectNodes: func(sourceId string, targetId string) { fmt.Fprint(ws, "c"+sourceId+targetId) },
        NodeOnline:      func(nodeId string) { fmt.Fprint(ws, "O"+nodeId) },
        NodeOffline:     func(nodeId string) { fmt.Fprint(ws, "o"+nodeId) },
    }

    server.Events.AddNode.Attach(eventHandlers.AddNode)
    server.Events.RemoveNode.Attach(eventHandlers.RemoveNode)
    server.Events.ConnectNodes.Attach(eventHandlers.ConnectNodes)
    server.Events.DisconnectNodes.Attach(eventHandlers.DisconnectNodes)
    server.Events.NodeOnline.Attach(eventHandlers.NodeOnline)
    server.Events.NodeOffline.Attach(eventHandlers.NodeOffline)

    go recordedevents.Replay(eventHandlers, 0)

    buf := make([]byte, 1)
    readFromWebsocket:
    for {
        if _, err := ws.Read(buf); err != nil {
            break readFromWebsocket
        }

        fmt.Fprint(ws, "_")
    }

    server.Events.AddNode.Detach(eventHandlers.AddNode)
    server.Events.RemoveNode.Detach(eventHandlers.RemoveNode)
    server.Events.ConnectNodes.Detach(eventHandlers.ConnectNodes)
    server.Events.DisconnectNodes.Detach(eventHandlers.DisconnectNodes)
    server.Events.NodeOnline.Detach(eventHandlers.NodeOnline)
    server.Events.NodeOffline.Detach(eventHandlers.NodeOffline)
}
