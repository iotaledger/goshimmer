package utxoutil

import (
	"bytes"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

func EqualAddresses(a1, a2 ledgerstate.Address) bool {
	if a1 == a2 {
		return true
	}
	if a1 == nil || a2 == nil {
		return false
	}
	if a1.Type() != a2.Type() {
		return false
	}
	if bytes.Compare(a1.Digest(), a2.Digest()) != 0 {
		return false
	}
	return true
}

func SigUnlockBlockED25519(essence *ledgerstate.TransactionEssence, keyPair *ed25519.KeyPair) *ledgerstate.SignatureUnlockBlock {
	addr := ledgerstate.NewED25519Address(keyPair.PublicKey)
	data := essence.Bytes()
	signature := ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(data))
	if !signature.AddressSignatureValid(addr, data) {
		panic("SigUnlockBlockED25519: internal error, signature invalid")
	}
	return ledgerstate.NewSignatureUnlockBlock(signature)
}
