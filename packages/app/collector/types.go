package collector

import (
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

// BlockType defines the component for the different BPS metrics.
type BlockType byte

const (
	// DataBlock denotes data block type.
	DataBlock BlockType = iota
	// Transaction denotes transaction block.
	Transaction
)

// String returns the stringified component type.
func (c BlockType) String() string {
	switch c {
	case DataBlock:
		return "DataBlock"
	case Transaction:
		return "Transaction"
	default:
		return "Unknown"
	}
}

func NewBlockType(payloadType payload.Type) BlockType {
	var blockType = DataBlock
	if payloadType == devnetvm.TransactionType {
		blockType = Transaction
	}
	return blockType
}
