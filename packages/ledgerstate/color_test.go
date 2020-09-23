package ledgerstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColoredBalancesFromBytes(t *testing.T) {
	coloredBalances := NewColoredBalances()
	coloredBalances.Set(ColorIOTA, 100)
	coloredBalances.Set(ColorMint, 120)

	restoredColoredBalances, _, err := ColoredBalancesFromBytes(coloredBalances.Bytes())
	require.NoError(t, err)

	iotaBalance, iotaColorExists := restoredColoredBalances.Get(ColorIOTA)
	mintBalance, mintColorExists := restoredColoredBalances.Get(ColorMint)

	assert.True(t, iotaColorExists)
	assert.Equal(t, uint64(100), iotaBalance)
	assert.True(t, mintColorExists)
	assert.Equal(t, uint64(120), mintBalance)
	assert.Equal(t, 2, restoredColoredBalances.Size())
}

func TestColoredBalances_String(t *testing.T) {
	coloredBalances := NewColoredBalances()
	coloredBalances.Set(ColorIOTA, 100)
	coloredBalances.Set(ColorMint, 120)

	assert.Equal(t, "ColoredBalances {\n    IOTA: 100\n    MINT: 120\n}", coloredBalances.String())
}
