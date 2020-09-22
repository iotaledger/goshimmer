package ledgerstate

import (
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region Color ////////////////////////////////////////////////////////////////////////////////////////////////////////

// ColorLength represents the length of a Color (amount of bytes).
const ColorLength = 32

// Color represents a marker that is associated to a token balance and that can give it a certain "meaning".
type Color [ColorLength]byte

// ColorFromBytes unmarshals a Color from a sequence of bytes.
func ColorFromBytes(bytes []byte) (color Color, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if color, err = ColorFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Color: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ColorFromBase58EncodedString creates a Color from a base58 encoded string.
func ColorFromBase58EncodedString(base58String string) (color Color, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded Color (%v): %w", err, ErrBase58DecodeFailed)
		return
	}

	if color, _, err = ColorFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse Color: %w", err)
		return
	}

	return
}

// ColorFromMarshalUtil parses a Color from the given MarshalUtil.
func ColorFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (color Color, err error) {
	colorBytes, err := marshalUtil.ReadBytes(ColorLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse Color (%v): %w", err, ErrParseBytesFailed)
		return
	}
	copy(color[:], colorBytes)

	return
}

// Bytes marshals the Color into a sequence of bytes.
func (c Color) Bytes() []byte {
	return c[:]
}

// Base58 returns a base58 encoded version of the Color.
func (c Color) Base58() string {
	return base58.Encode(c.Bytes())
}

// String creates a human readable string of the Color.
func (c Color) String() string {
	switch c {
	case ColorIOTA:
		return "IOTA"
	case ColorMint:
		return "MINT"
	default:
		return c.Base58()
	}
}

// ColorIOTA is the zero value of the Color and represents uncolored tokens.
var ColorIOTA = Color{}

// ColorMint represents a placeholder Color that indicates that tokens should be "colored" in their Output.
var ColorMint = Color{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ColoredBalances //////////////////////////////////////////////////////////////////////////////////////////////

// ColoredBalances represents a collection of balances associated to a certain Color.
type ColoredBalances struct {
	balances *orderedmap.OrderedMap
}

// NewColoredBalances returns an empty collection of ColoredBalances.
func NewColoredBalances() *ColoredBalances {
	return &ColoredBalances{balances: orderedmap.New()}
}

// ColoredBalancesFromBytes unmarshals ColoredBalances from a sequence of bytes.
func ColoredBalancesFromBytes(bytes []byte) (coloredBalances *ColoredBalances, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if coloredBalances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ColoredBalancesFromMarshalUtil parses ColoredBalances from the given MarshalUtil.
func ColoredBalancesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (coloredBalances *ColoredBalances, err error) {
	balancesCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to read element count (%v): %w", err, ErrParseBytesFailed)
		return
	}

	coloredBalances = NewColoredBalances()
	for i := uint32(0); i < balancesCount; i++ {
		color, colorErr := ColorFromMarshalUtil(marshalUtil)
		if colorErr != nil {
			err = xerrors.Errorf("failed to parse Color: %w", colorErr)
			return
		}

		balance, balanceErr := marshalUtil.ReadUint64()
		if balanceErr != nil {
			err = xerrors.Errorf("failed to parse balance (%v): %w", balanceErr, ErrParseBytesFailed)
			return
		}

		coloredBalances.Set(color, balance)
	}

	return
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

// Delete removes the given Color from the collection and returns true if it was removed.
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

// Bytes returns a marshaled version of the ColoredBalances.
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

// String returns a human readable version of the ColoredBalances.
func (c *ColoredBalances) String() string {
	structBuilder := stringify.StructBuilder("ColoredBalances")
	c.ForEach(func(color Color, balance uint64) bool {
		structBuilder.AddField(stringify.StructField(color.String(), balance))

		return true
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
