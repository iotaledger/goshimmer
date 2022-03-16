package ledger

import (
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type TransactionStoredEvent struct {
	Transaction         utxo.Transaction
	TransactionMetadata *TransactionMetadata
}

type TransactionSolidEvent struct {
	Inputs []utxo.Output
	
	propagationWalker *walker.Walker[utxo.TransactionID]

	*TransactionStoredEvent
}
