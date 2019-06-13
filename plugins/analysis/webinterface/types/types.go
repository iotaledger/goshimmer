package types

type EventHandlers = struct {
	AddNode         func(nodeId string)
	RemoveNode      func(nodeId string)
	ConnectNodes    func(sourceId string, targetId string)
	DisconnectNodes func(sourceId string, targetId string)
	NodeOnline      func(nodeId string)
	NodeOffline     func(nodeId string)
}

type EventHandlersConsumer = func(handler *EventHandlers)
