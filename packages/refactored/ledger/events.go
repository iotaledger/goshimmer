package ledger

import (
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type TransactionStoredEvent struct {
	TransactionID utxo.TransactionID
}

type TransactionBookedEvent struct {
	TransactionID utxo.TransactionID
	Outputs       Outputs
}

type TransactionForkedEvent struct {
	TransactionID  utxo.TransactionID
	ParentBranches utxo.TransactionIDs
}

type TransactionBranchIDUpdatedEvent struct {
	TransactionID    utxo.TransactionID
	AddedBranchID    utxo.TransactionID
	RemovedBranchIDs utxo.TransactionIDs
}
