package client

type EventDispatchers struct {
    AddNode         func(nodeId []byte)
    ConnectNodes    func(sourceId []byte, targetId []byte)
    DisconnectNodes func(sourceId []byte, targetId []byte)
}
