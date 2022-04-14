package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

func TestLoadSnapshot(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	ledgerState := tangle.Ledger

	wallets := createWallets(1)

	output := devnetvm.NewSigLockedColoredOutput(
		devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
			devnetvm.ColorIOTA: 10000,
		}),
		wallets[0].address,
	)

	genesisEssence := devnetvm.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		devnetvm.NewInputs(devnetvm.NewUTXOInput(utxo.NewOutputID(utxo.EmptyTransactionID, 0, []byte("")))),
		devnetvm.NewOutputs(output),
	)

	genesisTransaction := devnetvm.NewTransaction(genesisEssence, devnetvm.UnlockBlocks{devnetvm.NewReferenceUnlockBlock(0)})

	snapshot := &devnetvm.Snapshot{
		Transactions: map[utxo.TransactionID]devnetvm.Record{
			genesisTransaction.ID(): {
				Essence:        genesisEssence,
				UnlockBlocks:   devnetvm.UnlockBlocks{devnetvm.NewReferenceUnlockBlock(0)},
				UnspentOutputs: []bool{true},
			},
		},
	}

	ledgerState.LoadSnapshot(snapshot)
	assert.True(t, ledgerState.Storage.CachedTransactionMetadata(genesisTransaction.ID()).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
		assert.Equal(t, transactionMetadata.GradeOfFinality(), gof.High)
	}))
}
