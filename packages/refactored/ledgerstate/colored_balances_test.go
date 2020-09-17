package ledgerstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColoredBalances_String(t *testing.T) {
	coloredBalances := NewColoredBalances()
	coloredBalances.Set(IOTAColor, 100)
	coloredBalances.Set(MintColor, 120)

	assert.Equal(t, "ColoredBalances {\n    IOTA: 100\n    MINT: 120\n}", coloredBalances.String())
}
