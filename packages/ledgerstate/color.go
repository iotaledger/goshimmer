package ledgerstate

import (
	"bytes"
	"context"
	"sort"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
)

// region Color ////////////////////////////////////////////////////////////////////////////////////////////////////////

// ColorIOTA is the zero value of the Color and represents uncolored tokens.
var ColorIOTA = Color{}

// ColorMint represents a placeholder Color that indicates that tokens should be "colored" in their Output.
var ColorMint = Color{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

// ColorLength represents the length of a Color (amount of bytes).
const ColorLength = 32

// Color represents a marker that is associated to a token balance and that can give tokens a certain "meaning".
type Color [ColorLength]byte

// ColorFromBytes unmarshals a Color from a sequence of bytes.
func ColorFromBytes(data []byte) (color Color, consumedBytes int, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &color, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse SigLockedColoredOutput: %w", err)
		return
	}
	return
}

// ColorFromBase58EncodedString creates a Color from a base58 encoded string.
func ColorFromBase58EncodedString(base58String string) (color Color, err error) {
	parsedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded Color (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if color, _, err = ColorFromBytes(parsedBytes); err != nil {
		err = errors.Errorf("failed to parse Color from bytes: %w", err)
		return
	}

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

// String creates a human-readable string of the Color.
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

// Compare offers a comparator for Colors which returns -1 if otherColor is bigger, 1 if it is smaller and 0 if they are
// the same.
func (c Color) Compare(otherColor Color) int {
	return bytes.Compare(c[:], otherColor[:])
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ColoredBalances //////////////////////////////////////////////////////////////////////////////////////////////

// ColoredBalances represents a collection of balances associated to their respective Color that maintains a
// deterministic order of the present Colors.
type ColoredBalances struct {
	coloredBalancesInner `serix:"0"`
}

type coloredBalancesInner struct {
	Balances *orderedmap.OrderedMap[Color, uint64] `serix:"0,lengthPrefixType=uint32"`
}

// NewColoredBalances returns a new deterministically ordered collection of ColoredBalances.
func NewColoredBalances(balances map[Color]uint64) (coloredBalances *ColoredBalances) {
	coloredBalances = &ColoredBalances{coloredBalancesInner{Balances: orderedmap.New[Color, uint64]()}}

	// deterministically sort colors
	sortedColors := make([]Color, 0, len(balances))
	for color, balance := range balances {
		if balance == 0 {
			// drop zero balances
			continue
		}
		sortedColors = append(sortedColors, color)
	}
	sort.Slice(sortedColors, func(i, j int) bool { return sortedColors[i].Compare(sortedColors[j]) < 0 })

	// add sorted colors to the underlying map
	for _, color := range sortedColors {
		coloredBalances.Balances.Set(color, balances[color])
	}

	return
}

// ColoredBalancesFromBytes unmarshals ColoredBalances from a sequence of bytes.
func ColoredBalancesFromBytes(bytes []byte) (coloredBalances *ColoredBalances, consumedBytes int, err error) {
	coloredBalances = new(ColoredBalances)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, coloredBalances)
	if err != nil {
		err = errors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	return
}

// Get returns the balance of the given Color and a boolean value indicating if the requested Color existed.
func (c *ColoredBalances) Get(color Color) (uint64, bool) {
	return c.Balances.Get(color)
}

// ForEach calls the consumer for each element in the collection and aborts the iteration if the consumer returns false.
func (c *ColoredBalances) ForEach(consumer func(color Color, balance uint64) bool) {
	c.Balances.ForEach(consumer)
}

// Size returns the amount of individual balances in the ColoredBalances.
func (c *ColoredBalances) Size() int {
	return c.Balances.Size()
}

// Clone returns a copy of the ColoredBalances.
func (c *ColoredBalances) Clone() *ColoredBalances {
	copiedBalances := orderedmap.New[Color, uint64]()
	c.Balances.ForEach(copiedBalances.Set)

	return &ColoredBalances{
		coloredBalancesInner{
			Balances: copiedBalances,
		},
	}
}

// Bytes returns a marshaled version of the Marker.
func (c *ColoredBalances) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), c, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// Map returns a vanilla golang map (unordered) containing the existing balances. Since the ColoredBalances are
// immutable to ensure the deterministic ordering, this method can be used to retrieve a copy of the current values
// prior to some modification (like setting the updated colors of a minting transaction) which can then be used to
// create a new ColoredBalances object.
func (c *ColoredBalances) Map() (balances map[Color]uint64) {
	balances = make(map[Color]uint64)

	c.ForEach(func(color Color, balance uint64) bool {
		balances[color] = balance

		return true
	})

	return
}

// String returns a human-readable version of the ColoredBalances.
func (c *ColoredBalances) String() string {
	structBuilder := stringify.StructBuilder("ColoredBalances")
	c.ForEach(func(color Color, balance uint64) bool {
		structBuilder.AddField(stringify.StructField(color.String(), balance))

		return true
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
