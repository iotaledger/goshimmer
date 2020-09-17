package ledgerstate

import (
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// region ColoredBalances //////////////////////////////////////////////////////////////////////////////////////////////

type ColoredBalances struct {
	balances *orderedmap.OrderedMap
}

func NewColoredBalances() *ColoredBalances {
	return &ColoredBalances{balances: orderedmap.New()}
}

func (c *ColoredBalances) Set(color Color, balance uint64) bool {
	return c.balances.Set(color, balance)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Color ////////////////////////////////////////////////////////////////////////////////////////////////////////

// ColorLength represents the length of a Color (amount of bytes).
const ColorLength = 32

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
	switch color {
	case IOTAColor:
		return "IOTA"
	case MintColor:
		return "MINT"
	default:
		return base58.Encode(color[:])
	}
}

// ColorIOTA is the zero value of the Color and represents vanilla IOTA tokens.
var IOTAColor Color = [32]byte{}

// MintColor represents a placeholder Color that will be replaced with the transaction ID that created the funds. It is
// used to indicate that tokens should be "colored" in their Output (minting new colored coins).
var MintColor = [32]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
