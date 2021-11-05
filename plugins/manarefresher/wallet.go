package manarefresher

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// simple wallet with 1 keypair (there is only one address).
type wallet struct {
	keyPair ed25519.KeyPair
	address *ledgerstate.ED25519Address
}

// newWalletFromPrivateKey generates a simple wallet with one address.
func newWalletFromPrivateKey(pk ed25519.PrivateKey) *wallet {
	return &wallet{
		keyPair: ed25519.KeyPair{
			PrivateKey: pk,
			PublicKey:  pk.Public(),
		},
		address: ledgerstate.NewED25519Address(pk.Public()),
	}
}

func (w *wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w *wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func (w *wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), w.privateKey().Sign(txEssence.Bytes()))
}

// unlockBlocks returns the unlock blocks assuming all inputs can be unlocked by the same signature.
func (w *wallet) unlockBlocks(txEssence *ledgerstate.TransactionEssence) []ledgerstate.UnlockBlock {
	signatureUnlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	unlockBlocks[0] = signatureUnlockBlock
	for i := range txEssence.Inputs() {
		if i == 0 {
			continue
		}
		unlockBlocks[i] = ledgerstate.NewReferenceUnlockBlock(0)
	}
	return unlockBlocks
}
