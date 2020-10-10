package ledgerstate

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

func TestTransaction_Bytes(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)

	essence := NewTransactionEssence(0,
		NewInputs(
			NewUTXOInput(NewOutputID(GenesisTransactionID, 12)),
			NewUTXOInput(NewOutputID(GenesisTransactionID, 10)),
			NewUTXOInput(NewOutputID(GenesisTransactionID, 1)),
		),
	)

	signature := privateKey.Sign(essence.Bytes())

	transaction := NewTransaction(
		essence,
		NewUnlockBlocks(
			NewSignatureUnlockBlock(NewED25519Signature(publicKey, signature)),
			NewReferenceUnlockBlock(0),
			NewSignatureUnlockBlock(NewED25519Signature(publicKey, signature)),
			NewSignatureUnlockBlock(NewED25519Signature(publicKey, signature)),
		),
	)

	fmt.Print(transaction)
}
