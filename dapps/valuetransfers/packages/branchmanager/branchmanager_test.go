package branchmanager

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/testutil"
)

func TestSomething(t *testing.T) {
	branchManager := New(testutil.DB(t))

	cachedBranch1, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{transaction.OutputID{4}})
	defer cachedBranch1.Release()
	_ = cachedBranch1.Unwrap()

	cachedBranch2, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{transaction.OutputID{4}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	fmt.Println(branchManager.BranchesConflicting(MasterBranchID, branch2.ID()))
}
