package types

// EventHandlers holds the handler for each event.
type EventHandlers = struct {
	// Addnode defines the handler called when adding a new node.
	AddNode func(nodeId string)
	// RemoveNode defines the handler called when adding removing a node.
	RemoveNode func(nodeId string)
	// ConnectNodes defines the handler called when connecting two nodes.
	ConnectNodes func(sourceId string, targetId string)
	// DisconnectNodes defines the handler called when connecting two nodes.
	DisconnectNodes func(sourceId string, targetId string)
}

// EventHandlersConsumer defines the consumer function of an *EventHandlers.
type EventHandlersConsumer = func(handler *EventHandlers)
