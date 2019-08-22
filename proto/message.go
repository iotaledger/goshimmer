package proto

import "github.com/golang/protobuf/proto"

// Enum for the different message types.
const (
	MPing MType = iota
	MPong
)

// Mtype is the type of message type enum.
type MType int

// Message extends the proto.Message interface with additional util functions.
type Message interface {
	proto.Message

	// Name returns the name of the corresponding message type for debugging.
	Name() string
	// Type returns the type of the corresponding message as an enum.
	Type() MType

	// Wrapper returns the corresponding wrapped message.
	Wrapper() *MessageWrapper
}

func (m *Ping) Name() string { return "PING" }
func (m *Ping) Type() MType  { return MPing }
func (m *Ping) Wrapper() *MessageWrapper {
	return &MessageWrapper{Message: &MessageWrapper_Ping{Ping: m}}
}

func (m *Pong) Name() string { return "PONG" }
func (m *Pong) Type() MType  { return MPong }
func (m *Pong) Wrapper() *MessageWrapper {
	return &MessageWrapper{Message: &MessageWrapper_Pong{Pong: m}}
}
