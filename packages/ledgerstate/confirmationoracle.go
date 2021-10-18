package ledgerstate

import (
	"github.com/iotaledger/hive.go/datastructure/walker"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
)

// region SimpleConfirmationOracle /////////////////////////////////////////////////////////////////////////////////////

// SimpleConfirmationOracle is a very simple ConfirmationOracle that retrieves the confirmation status by analyzing the
// gradeOfFinality of the different entities.
type SimpleConfirmationOracle struct {
	ledgerstate *Ledgerstate
}

// NewSimpleConfirmationOracle is the constructor for the SimpleConfirmationOracle.
func NewSimpleConfirmationOracle(ledgerstate *Ledgerstate) *SimpleConfirmationOracle {
	return &SimpleConfirmationOracle{
		ledgerstate: ledgerstate,
	}
}

// IsTransactionConfirmed returns true if the transaction has been accepted to stay in the ledger.
func (s *SimpleConfirmationOracle) IsTransactionConfirmed(transactionID TransactionID) (isConfirmed bool) {
	s.ledgerstate.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		isConfirmed = transactionMetadata.GradeOfFinality() >= gof.High
	})

	return
}

// IsTransactionRejected returns true if the transaction will not stay part of the ledger permanently.
func (s *SimpleConfirmationOracle) IsTransactionRejected(transactionID TransactionID) (isRejected bool) {
	s.ledgerstate.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		isRejected = s.IsBranchRejected(transactionMetadata.BranchID())
	})

	return
}

// IsBranchConfirmed returns true if the Branch has been accepted to stay in the ledger.
func (s *SimpleConfirmationOracle) IsBranchConfirmed(branchID BranchID) bool {
	return s.IsTransactionConfirmed(branchID.TransactionID())
}

// IsBranchRejected returns true if the Branch will not stay part of the ledger permanently.
func (s *SimpleConfirmationOracle) IsBranchRejected(branchID BranchID) (isRejected bool) {
	branchWalker := walker.New(false)
	branchWalker.Push(branchID)
	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next().(BranchID)
		if isRejected = s.isConflictingBranchConfirmed(currentBranchID); isRejected {
			return
		}

		s.queueIsBranchRejectedParents(currentBranchID, branchWalker)
	}

	return
}

func (s *SimpleConfirmationOracle) queueIsBranchRejectedParents(currentBranchID BranchID, branchWalker *walker.Walker) {
	conflictingBranchIDs, err := s.ledgerstate.ResolveConflictBranchIDs(NewBranchIDs(currentBranchID))
	if err != nil {
		return
	}

	for conflictBranchID := range conflictingBranchIDs {
		s.ledgerstate.Branch(conflictBranchID).Consume(func(branch Branch) {
			for parentBranchID := range branch.Parents() {
				branchWalker.Push(parentBranchID)
			}
		})
	}
}

func (s *SimpleConfirmationOracle) isConflictingBranchConfirmed(currentBranchID BranchID) (isRejected bool) {
	s.ledgerstate.ForEachConflictingBranchID(currentBranchID, func(conflictingBranchID BranchID) {
		if isRejected {
			return
		}

		isRejected = s.IsTransactionConfirmed(conflictingBranchID.TransactionID())
	})

	return
}

// code contract (make sure the struct implements all required methods).
var _ ConfirmationOracle = &SimpleConfirmationOracle{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConfirmationOracle ///////////////////////////////////////////////////////////////////////////////////////////

// ConfirmationOracle is an interface for the component that answers questions about the confirmation status of the
// different entities of the ledger.
type ConfirmationOracle interface {
	// IsTransactionConfirmed returns true if the transaction has been accepted to stay in the ledger.
	IsTransactionConfirmed(transactionID TransactionID) bool

	// IsTransactionRejected returns true if the transaction will not stay part of the ledger permanently.
	IsTransactionRejected(transactionID TransactionID) bool

	// IsBranchConfirmed returns true if the Branch has been accepted to stay in the ledger.
	IsBranchConfirmed(branchID BranchID) bool

	// IsBranchRejected returns true if the Branch will not stay part of the ledger permanently.
	IsBranchRejected(branchID BranchID) bool
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
