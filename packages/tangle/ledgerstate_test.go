package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSnapshot(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()

	ledgerState := tangle.LedgerState

	wallets := createWallets(1)

	snapshot := map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: {
			wallets[0].address: ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: 10000,
				})},
	}

	ledgerState.LoadSnapshot(snapshot)
	inclusionState, err := ledgerState.TransactionInclusionState(ledgerstate.GenesisTransactionID)
	require.NoError(t, err)
	assert.Equal(t, ledgerstate.Confirmed, inclusionState)
}
