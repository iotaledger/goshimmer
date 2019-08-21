package proto

import "github.com/golang/protobuf/proto"

// MessageType is the enum type for the different types of messages.
type MessageType int

const (
	PING MessageType = iota + 1
	PONG
)

// Message extends the proto.Message interface to provide additional util functions.
type Message interface {
	proto.Message

	// Name returns the name of the corresponding message type for debugging.
	Name() string
	// Type returns the type of the corresponding message as an enum.
	Type() MessageType

	// Wrapper returns the corresponding wrapped message.
	Wrapper() *MessageWrapper
}

func (m *Ping) Name() string      { return "PING" }
func (m *Ping) Type() MessageType { return PING }
func (m *Ping) Wrapper() *MessageWrapper {
	return &MessageWrapper{Message: &MessageWrapper_Ping{Ping: m}}
}

func (m *Pong) Name() string      { return "PONG" }
func (m *Pong) Type() MessageType { return PONG }
func (m *Pong) Wrapper() *MessageWrapper {
	return &MessageWrapper{Message: &MessageWrapper_Pong{Pong: m}}
}
