package valuetransfers

import (
	"github.com/mr-tron/base58"
)

type TransferId [TransferIdLength]byte

func NewTransferId(idBytes []byte) (result TransferId) {
	copy(result[:], idBytes)

	return
}

func (id TransferId) String() string {
	return base58.Encode(id[:])
}

const TransferIdLength = 32
