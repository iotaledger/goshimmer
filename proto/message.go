package proto

import "github.com/golang/protobuf/proto"

type MessageType int

const (
	PING MessageType = iota + 1
	PONG
)

type Message interface {
	proto.Message

	Name() string
	Type() MessageType

	// Returns the wrapped message
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
