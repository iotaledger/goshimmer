package ledgerstate

import (
	"fmt"
	"testing"
)

func TestTransaction_Bytes(t *testing.T) {
	transaction := NewTransaction(
		NewTransactionEssence(
			0,
			NewInputs(
				NewUTXOInput(NewOutputID(GenesisTransactionID, 12)),
				NewUTXOInput(NewOutputID(GenesisTransactionID, 10)),
				NewUTXOInput(NewOutputID(GenesisTransactionID, 1)),
			),
		),
		NewUnlockBlocks(
			NewReferenceUnlockBlock(2),
			NewReferenceUnlockBlock(1),
		),
	)

	fmt.Print(transaction)
}
