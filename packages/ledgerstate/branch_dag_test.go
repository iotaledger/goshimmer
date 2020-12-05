package ledgerstate

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func TestBranchDAG_ConflictBranches(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	defer branchDAG.Shutdown()

	conflictBranch, newBranchCreated := branchDAG.CreateConflictBranch(
		NewBranchID(TransactionID{3}),
		NewBranchIDs(
			NewBranchID(TransactionID{1}),
		),
		NewConflictIDs(
			NewConflictID(NewOutputID(TransactionID{2}, 0)),
			NewConflictID(NewOutputID(TransactionID{2}, 1)),
		),
	)
	defer conflictBranch.Release()
	fmt.Println(conflictBranch, newBranchCreated)

	conflictBranch1, newBranchCreated1 := branchDAG.CreateConflictBranch(
		NewBranchID(TransactionID{3}),
		NewBranchIDs(
			NewBranchID(TransactionID{1}),
		),
		NewConflictIDs(
			NewConflictID(NewOutputID(TransactionID{2}, 0)),
			NewConflictID(NewOutputID(TransactionID{2}, 1)),
			NewConflictID(NewOutputID(TransactionID{2}, 2)),
		),
	)
	defer conflictBranch1.Release()
	fmt.Println(conflictBranch1, newBranchCreated1)
}
