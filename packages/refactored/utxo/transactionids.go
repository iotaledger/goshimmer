package utxo

import (
	"github.com/iotaledger/hive.go/generics/set"
)

type TransactionIDs = *set.AdvancedSet[*TransactionID]

func NewTransactionIDs(ids ...*TransactionID) (new TransactionIDs) {
	return set.NewAdvancedSet[*TransactionID](ids...)
}
