package devnetvm

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/lo"
)

type wallet struct {
	keyPair ed25519.KeyPair
	address *ED25519Address
}

func genRandomWallet() wallet {
	kp := ed25519.GenerateKeyPair()
	return wallet{
		kp,
		NewED25519Address(kp.PublicKey),
	}
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func createWallets(n int) []wallet {
	wallets := make([]wallet, n)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *TransactionEssence) *ED25519Signature {
	return NewED25519Signature(w.publicKey(), w.privateKey().Sign(lo.PanicOnErr(txEssence.Bytes())))
}

func (w wallet) unlockBlocks(txEssence *TransactionEssence) []UnlockBlock {
	unlockBlock := NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}
