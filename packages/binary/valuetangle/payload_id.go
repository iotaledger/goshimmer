package valuetangle

import (
	"github.com/mr-tron/base58"
)

type PayloadId [PayloadIdLength]byte

func NewPayloadId(idBytes []byte) (result PayloadId) {
	copy(result[:], idBytes)

	return
}

func (id PayloadId) String() string {
	return base58.Encode(id[:])
}

var EmptyPayloadId PayloadId

const PayloadIdLength = 32
