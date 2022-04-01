package ledger

import (
	utxo2 "github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

type TransactionStoredEvent struct {
	TransactionID utxo2.TransactionID
}

type TransactionBookedEvent struct {
	TransactionID utxo2.TransactionID
	Outputs       Outputs
}

type TransactionForkedEvent struct {
	TransactionID  utxo2.TransactionID
	ParentBranches utxo2.TransactionIDs
}

type TransactionBranchIDUpdatedEvent struct {
	TransactionID    utxo2.TransactionID
	AddedBranchID    utxo2.TransactionID
	RemovedBranchIDs utxo2.TransactionIDs
}
