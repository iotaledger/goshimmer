package ledgerstate

import (
	"github.com/iotaledger/hive.go/datastructure/set"
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
	utxoDAG   *UTXODAG
	branchDAG *BranchDAG
}

func (s *SimpleConfirmationOracle) IsTransactionConfirmed(transactionID TransactionID) (isConfirmed bool) {
	s.utxoDAG.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		isConfirmed = transactionMetadata.GradeOfFinality() >= gof.Medium
	})

	return
}

func NewSimpleConfirmationOracle(utxoDAG *UTXODAG, branchDAG *BranchDAG) *SimpleConfirmationOracle {
	return &SimpleConfirmationOracle{
		utxoDAG:   utxoDAG,
		branchDAG: branchDAG,
	}
}

func (s *SimpleConfirmationOracle) IsTransactionRejected(transactionID TransactionID) (isRejected bool) {
	s.utxoDAG.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		isRejected = s.IsBranchRejected(transactionMetadata.BranchID())
	})

	return
}

func (s *SimpleConfirmationOracle) IsBranchConfirmed(branchID BranchID) bool {
	return s.IsTransactionConfirmed(branchID.TransactionID())
}

func (s *SimpleConfirmationOracle) IsBranchRejected(branchID BranchID) (isRejected bool) {
	seenBranches := set.New()

	branchWalker := walker.New(false)
	branchWalker.Push(branchID)

	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next().(BranchID)

		s.branchDAG.ForEachConflictingBranchID(currentBranchID, func(conflictingBranchID BranchID) {
			if isRejected {
				return
			}

			isRejected = s.IsTransactionConfirmed(conflictingBranchID.TransactionID())
		})

		if isRejected {
			return
		}

		conflictingBranchIDs, err := s.branchDAG.ResolveConflictBranchIDs(NewBranchIDs(branchID))
		if err != nil {
			return
		}

		for conflictBranchID := range conflictingBranchIDs {
			s.branchDAG.Branch(conflictBranchID).Consume(func(branch Branch) {
				for parentBranchID := range branch.Parents() {
					if !seenBranches.Add(parentBranchID) {
						continue
					}

					branchWalker.Push(parentBranchID)
				}
			})
		}
	}

	return
}

var _ ConfirmationOracle = &SimpleConfirmationOracle{}
