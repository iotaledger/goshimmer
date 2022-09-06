package network

type Network interface {
	// Broadcast broadcasts a message to all connected peers.
	Broadcast(msg []byte) error

	// Send sends a message to a specific peer.
	Send(peerID string, msg []byte) error

	OnReceive(callback func(peerID string, msg []byte))
}
