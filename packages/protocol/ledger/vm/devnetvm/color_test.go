package devnetvm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"
)

func TestNewColoredBalances(t *testing.T) {
	coloredBalances := NewColoredBalances(map[Color]uint64{
		{3}: 100,
		{0}: 200,
		{1}: 300,
		{2}: 400,
	})

	orderedKeys := make([]Color, 0)
	orderedBalances := make([]uint64, 0)
	coloredBalances.ForEach(func(color Color, balance uint64) bool {
		orderedKeys = append(orderedKeys, color)
		orderedBalances = append(orderedBalances, balance)

		return true
	})
	require.Equal(t, []Color{{0}, {1}, {2}, {3}}, orderedKeys)
	require.Equal(t, []uint64{200, 300, 400, 100}, orderedBalances)
}

func TestColoredBalancesFromBytes(t *testing.T) {
	coloredBalances := NewColoredBalances(map[Color]uint64{
		{3}: 100,
		{0}: 200,
		{1}: 300,
		{2}: 400,
	})
	marshaledColoredBalances := coloredBalances.Bytes()

	clonedColoredBalances, consumedBytes, err := ColoredBalancesFromBytes(marshaledColoredBalances)
	require.NoError(t, err)
	require.Equal(t, len(marshaledColoredBalances), consumedBytes)
	require.Equal(t, clonedColoredBalances.Size(), coloredBalances.Size())
	require.Equal(t, clonedColoredBalances.Bytes(), coloredBalances.Bytes())

	color0Balance, color0Exists := clonedColoredBalances.Get(Color{0})
	require.True(t, color0Exists)
	require.Equal(t, uint64(200), color0Balance)

	color1Balance, color1Exists := clonedColoredBalances.Get(Color{1})
	require.True(t, color1Exists)
	require.Equal(t, uint64(300), color1Balance)

	color2Balance, color2Exists := clonedColoredBalances.Get(Color{2})
	require.True(t, color2Exists)
	require.Equal(t, uint64(400), color2Balance)

	color3Balance, color3Exists := clonedColoredBalances.Get(Color{3})
	require.True(t, color3Exists)
	require.Equal(t, uint64(100), color3Balance)
}

func TestColoredBalances_String(t *testing.T) {
	coloredBalances := NewColoredBalances(map[Color]uint64{
		ColorIOTA: 100,
		ColorMint: 120,
	})

	require.Equal(t, "ColoredBalances {\n    IOTA: 100\n    MINT: 120\n}", coloredBalances.String())
}

func TestNonExistentColor(t *testing.T) {
	h := blake2b.Sum256([]byte("dummy"))
	color, _, err := ColorFromBytes(h[:])
	require.NoError(t, err)
	m := map[Color]uint64{color: 5}

	cb := NewColoredBalances(m)
	amount, ok := cb.Get(color)
	require.EqualValues(t, 5, amount)
	require.True(t, ok)

	require.NotPanics(t, func() {
		amount, ok = cb.Get(ColorIOTA)
	})
	require.False(t, ok)
	require.EqualValues(t, 0, amount)
}
