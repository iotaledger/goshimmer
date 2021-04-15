package tangle

import (
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/identity"
)

func TestLoadSnapshot(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()

	ledgerState := tangle.LedgerState

	wallets := createWallets(1)

	output := ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 10000,
		}),
		wallets[0].address,
	)

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]*ledgerstate.TransactionEssence{
			ledgerstate.GenesisTransactionID: ledgerstate.NewTransactionEssence(
				0,
				time.Now(),
				identity.ID{},
				identity.ID{},
				ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
				ledgerstate.NewOutputs(output),
			)},
	}

	ledgerState.LoadSnapshot(snapshot)
	inclusionState, err := ledgerState.TransactionInclusionState(ledgerstate.GenesisTransactionID)
	require.NoError(t, err)
	assert.Equal(t, ledgerstate.Confirmed, inclusionState)
}
