package utxotest

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"
	"testing"
)

func TestNonExistentColor(t *testing.T) {
	h := blake2b.Sum256([]byte("dummy"))
	color, _, err := ledgerstate.ColorFromBytes(h[:])
	require.NoError(t, err)
	m := map[ledgerstate.Color]uint64{color: 5}

	cb := ledgerstate.NewColoredBalances(m)
	amount, ok := cb.Get(color)
	require.EqualValues(t, 5, amount)
	require.True(t, ok)

	require.NotPanics(t, func() {
		amount, ok = cb.Get(ledgerstate.ColorIOTA)
	})
	require.False(t, ok)
	require.EqualValues(t, 0, amount)
}
