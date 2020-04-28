package balance

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// Color represents a marker that is associated to a token balance and that gives it a certain "meaning". The zero value
// represents "vanilla" IOTA tokens but it is also possible to define tokens that represent i.e. real world assets.
type Color [ColorLength]byte

// ColorFromBytes unmarshals a Color from a sequence of bytes.
func ColorFromBytes(bytes []byte) (result Color, consumedBytes int, err error) {
	colorBytes, err := marshalutil.New(bytes).ReadBytes(ColorLength)
	if err != nil {
		return
	}
	copy(result[:], colorBytes)

	consumedBytes = ColorLength

	return
}

// Bytes marshals the Color into a sequence of bytes.
func (color Color) Bytes() []byte {
	return color[:]
}

// String creates a human readable string of the Color.
func (color Color) String() string {
	if color == ColorIOTA {
		return "IOTA"
	}

	return base58.Encode(color[:])
}

// ColorLength represents the length of a Color (amount of bytes).
const ColorLength = 32

// ColorIOTA is the zero value of the Color and represents vanilla IOTA tokens.
var ColorIOTA Color = [32]byte{}

// ColorNew represents a placeholder Color that will be replaced with the transaction ID that created the funds. It is
// used to indicate that tokens should be "colored" in their Output (minting new colored coins).
var ColorNew = [32]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
