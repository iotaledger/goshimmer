package server

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/peer"
)

// MType is the type of message type enum.
type MType uint

// The Sender interface specifies common method required to send requests.
type Sender interface {
	LocalAddr() string

	Send(toAddr string, data []byte)
	SendExpectingReply(toAddr string, toID peer.ID, data []byte, replyType MType, callback func(interface{}) bool) <-chan error
}

// A Handler reacts to an incoming message.
type Handler interface {
	// HandleMessage is called for each incoming message.
	// It returns true, if that particular message type can be processed by the current Handler.
	HandleMessage(s *Server, from *peer.Peer, data []byte) (bool, error)
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions as Server handlers.
// If f is a function with the appropriate signature, HandlerFunc(f) is a Handler that calls f.
type HandlerFunc func(*Server, *peer.Peer, []byte) (bool, error)

// HandleMessage returns f(s, from, data).
func (f HandlerFunc) HandleMessage(s *Server, from *peer.Peer, data []byte) (bool, error) {
	return f(s, from, data)
}

// PacketHash returns the hash of a packet
func PacketHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
