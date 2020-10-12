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

	outputsToSpend := NewOutputs(
		NewSigLockedSingleOutput(
			1337,
			NewED25519Address(publicKey),
		).SetID(
			NewOutputID(GenesisTransactionID, 1),
		),

		NewSigLockedColoredOutput(
			NewColoredBalances(map[Color]uint64{
				Color{2}: 100,
			}),
			NewED25519Address(publicKey),
		).SetID(
			NewOutputID(GenesisTransactionID, 2),
		),
	)

	essence := NewTransactionEssence(
		0,
		outputsToSpend.Inputs(),
		NewOutputs(
			NewSigLockedColoredOutput(
				NewColoredBalances(map[Color]uint64{
					ColorIOTA: 1337,
					Color{2}:  100,
				}),
				NewED25519Address(publicKey),
			),
		),
	)

	transaction := NewTransaction(
		essence,
		NewUnlockBlocks(
			NewSignatureUnlockBlock(NewED25519Signature(publicKey, privateKey.Sign(essence.Bytes()))),
			NewReferenceUnlockBlock(0),
		),
	)

	fmt.Print(transaction)

	transaction.IsSyntacticallyValid()
	transaction.IsSemanticallyValid()

	clonedTransaction, _, err := TransactionFromBytes(transaction.Bytes())
	require.NoError(t, err)

	fmt.Print(clonedTransaction)
}
