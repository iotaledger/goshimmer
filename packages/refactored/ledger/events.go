package ledger

import (
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
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
	ParentBranches branchdag.BranchIDs
}

type TransactionBranchIDUpdatedEvent struct {
	TransactionID    utxo.TransactionID
	AddedBranchID    branchdag.BranchID
	RemovedBranchIDs branchdag.BranchIDs
}
