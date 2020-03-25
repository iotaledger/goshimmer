package client

type EventDispatchers struct {
	Heartbeat func(nodeId []byte, outboundIds [][]byte, inboundIds [][]byte)
}
