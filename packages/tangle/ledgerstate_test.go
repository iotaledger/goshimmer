package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(epochs.DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(output),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]*ledgerstate.TransactionEssence{
			genesisTransaction.ID(): genesisEssence,
		},
	}

	ledgerState.LoadSnapshot(snapshot)
	inclusionState, err := ledgerState.TransactionInclusionState(genesisTransaction.ID())
	require.NoError(t, err)
	assert.Equal(t, ledgerstate.Confirmed, inclusionState)
}
