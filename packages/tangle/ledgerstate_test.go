package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
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

type wallet struct {
	keyPair ed25519.KeyPair
	address *ledgerstate.ED25519Address
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func createWallets(n int) []wallet {
	wallets := make([]wallet, 2)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			ledgerstate.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), ed25519.Signature(w.privateKey().Sign(txEssence.Bytes())))
}

func (w wallet) unlockBlocks(txEssence *ledgerstate.TransactionEssence) []ledgerstate.UnlockBlock {
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}
