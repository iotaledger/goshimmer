package proto

import (
	"github.com/golang/protobuf/proto"
)

// MType is the type of message type enum.
type MType uint

// An enum for the different message types.
const (
	MTransaction MType = 20 + iota
	MTransactionRequest
)

// Message extends the proto.Message interface with additional util functions.
type Message interface {
	proto.Message

	// Name returns the name of the corresponding message type for debugging.
	Name() string
	// Type returns the type of the corresponding message as an enum.
	Type() MType
}

func (m *Transaction) Name() string { return "TRANSACTION" }
func (m *Transaction) Type() MType  { return MTransaction }

func (m *TransactionRequest) Name() string { return "TRANSACTION_REQUEST" }
func (m *TransactionRequest) Type() MType  { return MTransactionRequest }
