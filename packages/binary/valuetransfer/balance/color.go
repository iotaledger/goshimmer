package balance

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type Color [ColorLength]byte

func ColorFromBytes(bytes []byte) (result Color, err error, consumedBytes int) {
	colorBytes, err := marshalutil.New(bytes).ReadBytes(ColorLength)
	if err != nil {
		return
	}
	copy(result[:], colorBytes)

	consumedBytes = ColorLength

	return
}

const ColorLength = 32

func (color Color) Bytes() []byte {
	return color[:]
}

func (color Color) String() string {
	if color == COLOR_IOTA {
		return "IOTA"
	}

	return base58.Encode(color[:])
}

var COLOR_IOTA Color = [32]byte{}
