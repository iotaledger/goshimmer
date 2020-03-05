package color

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type Color [Length]byte

func FromBytes(bytes []byte) (result Color, err error, consumedBytes int) {
	colorBytes, err := marshalutil.New(bytes).ReadBytes(Length)
	if err != nil {
		return
	}
	copy(result[:], colorBytes)

	consumedBytes = Length

	return
}

const Length = 32

func (color Color) Bytes() []byte {
	return color[:]
}

func (color Color) String() string {
	if color == IOTA {
		return "IOTA"
	}

	return base58.Encode(color[:])
}

var IOTA Color = [32]byte{}
