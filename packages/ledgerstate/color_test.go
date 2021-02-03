package ledgerstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, []Color{{0}, {1}, {2}, {3}}, orderedKeys)
	assert.Equal(t, []uint64{200, 300, 400, 100}, orderedBalances)
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
	assert.Equal(t, len(marshaledColoredBalances), consumedBytes)
	assert.Equal(t, clonedColoredBalances.Size(), coloredBalances.Size())
	assert.Equal(t, clonedColoredBalances.Bytes(), coloredBalances.Bytes())

	color0Balance, color0Exists := clonedColoredBalances.Get(Color{0})
	assert.True(t, color0Exists)
	assert.Equal(t, uint64(200), color0Balance)

	color1Balance, color1Exists := clonedColoredBalances.Get(Color{1})
	assert.True(t, color1Exists)
	assert.Equal(t, uint64(300), color1Balance)

	color2Balance, color2Exists := clonedColoredBalances.Get(Color{2})
	assert.True(t, color2Exists)
	assert.Equal(t, uint64(400), color2Balance)

	color3Balance, color3Exists := clonedColoredBalances.Get(Color{3})
	assert.True(t, color3Exists)
	assert.Equal(t, uint64(100), color3Balance)
}

func TestColoredBalances_String(t *testing.T) {
	coloredBalances := NewColoredBalances(map[Color]uint64{
		ColorIOTA: 100,
		ColorMint: 120,
	})

	assert.Equal(t, "ColoredBalances {\n    IOTA: 100\n    MINT: 120\n}", coloredBalances.String())
}
