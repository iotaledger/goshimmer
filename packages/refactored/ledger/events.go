package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/utxo"
)

type Events struct {
	TransactionStored          *event.Event[*TransactionStoredEvent]
	TransactionBooked          *event.Event[*TransactionBookedEvent]
	TransactionForked          *event.Event[*TransactionForkedEvent]
	TransactionBranchIDUpdated *event.Event[*TransactionBranchIDUpdatedEvent]
	Error                      *event.Event[error]
}

func NewEvents() (new *Events) {
	return &Events{
		TransactionStored:          event.New[*TransactionStoredEvent](),
		TransactionBooked:          event.New[*TransactionBookedEvent](),
		TransactionForked:          event.New[*TransactionForkedEvent](),
		TransactionBranchIDUpdated: event.New[*TransactionBranchIDUpdatedEvent](),
		Error:                      event.New[error](),
	}
}

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
