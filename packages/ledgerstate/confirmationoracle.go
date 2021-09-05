package ledgerstate

import (
	"github.com/iotaledger/hive.go/datastructure/walker"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
)

// ConfirmationOracle answers questions about entities' confirmation.
type ConfirmationOracle interface {
	IsTransactionConfirmed(transactionID TransactionID) bool
	IsTransactionRejected(transactionID TransactionID) bool
	IsBranchConfirmed(branchID BranchID) bool
	IsBranchRejected(branchID BranchID) bool
}

type SimpleConfirmationOracle struct {
	ledgerstate *Ledgerstate
}

func (s *SimpleConfirmationOracle) IsTransactionConfirmed(transactionID TransactionID) (isConfirmed bool) {
	s.ledgerstate.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		isConfirmed = transactionMetadata.GradeOfFinality() >= gof.Medium
	})

	return
}

func NewSimpleConfirmationOracle(ledgerstate *Ledgerstate) *SimpleConfirmationOracle {
	return &SimpleConfirmationOracle{
		ledgerstate: ledgerstate,
	}
}

func (s *SimpleConfirmationOracle) IsTransactionRejected(transactionID TransactionID) (isRejected bool) {
	s.ledgerstate.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		isRejected = s.IsBranchRejected(transactionMetadata.BranchID())
	})

	return
}

func (s *SimpleConfirmationOracle) IsBranchConfirmed(branchID BranchID) bool {
	return s.IsTransactionConfirmed(branchID.TransactionID())
}

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

var _ ConfirmationOracle = &SimpleConfirmationOracle{}
