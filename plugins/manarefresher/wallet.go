package manarefresher

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/lo"

	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

// simple wallet with 1 keypair (there is only one address).
type wallet struct {
	keyPair ed25519.KeyPair
	address *devnetvm.ED25519Address
}

// newWalletFromPrivateKey generates a simple wallet with one address.
func newWalletFromPrivateKey(pk ed25519.PrivateKey) *wallet {
	return &wallet{
		keyPair: ed25519.KeyPair{
			PrivateKey: pk,
			PublicKey:  pk.Public(),
		},
		address: devnetvm.NewED25519Address(pk.Public()),
	}
}

func (w *wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w *wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func (w *wallet) sign(txEssence *devnetvm.TransactionEssence) *devnetvm.ED25519Signature {
	return devnetvm.NewED25519Signature(w.publicKey(), w.privateKey().Sign(lo.PanicOnErr(txEssence.Bytes())))
}

// unlockBlocks returns the unlock blocks assuming all inputs can be unlocked by the same signature.
func (w *wallet) unlockBlocks(txEssence *devnetvm.TransactionEssence) []devnetvm.UnlockBlock {
	signatureUnlockBlock := devnetvm.NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]devnetvm.UnlockBlock, len(txEssence.Inputs()))
	unlockBlocks[0] = signatureUnlockBlock
	for i := range txEssence.Inputs() {
		if i == 0 {
			continue
		}
		unlockBlocks[i] = devnetvm.NewReferenceUnlockBlock(0)
	}
	return unlockBlocks
}
