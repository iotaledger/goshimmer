package ledgerstate

import (
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// region ColoredBalances //////////////////////////////////////////////////////////////////////////////////////////////

// ColoredBalances represents a collection of balances associated to a certain Color.
type ColoredBalances struct {
	balances *orderedmap.OrderedMap
}

// NewColoredBalances returns an empty collection of ColoredBalances.
func NewColoredBalances() *ColoredBalances {
	return &ColoredBalances{balances: orderedmap.New()}
}

// Set sets the balance of the given Color.
func (c *ColoredBalances) Set(color Color, balance uint64) bool {
	return c.balances.Set(color, balance)
}

// Get returns the balance of the given Color and a boolean value indicating if the requested Color existed.
func (c *ColoredBalances) Get(color Color) (uint64, bool) {
	balance, exists := c.balances.Get(color)

	return balance.(uint64), exists
}

// Delete removes the given Color from the collection and returns true if the Color was found.
func (c *ColoredBalances) Delete(color Color) bool {
	return c.balances.Delete(color)
}

// ForEach calls the consumer for each element in the collection and aborts the iteration if the consumer returns false.
func (c *ColoredBalances) ForEach(consumer func(color Color, balance uint64) bool) {
	c.balances.ForEach(func(key, value interface{}) bool {
		return consumer(key.(Color), value.(uint64))
	})
}

// Size returns the amount of individual balances in the ColoredBalances.
func (c *ColoredBalances) Size() int {
	return c.balances.Size()
}

func (c *ColoredBalances) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(c.balances.Size()))
	c.ForEach(func(color Color, balance uint64) bool {
		marshalUtil.WriteBytes(color.Bytes())
		marshalUtil.WriteUint64(balance)

		return true
	})
	return marshalUtil.Bytes()
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
var IOTAColor = Color{}

// MintColor represents a placeholder Color that will be replaced with the transaction ID that created the funds. It is
// used to indicate that tokens should be "colored" in their Output (minting new colored coins).
var MintColor = Color{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
