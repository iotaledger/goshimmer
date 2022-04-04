package branchdag

type utils struct {
	branchDAG *BranchDAG
}

func newUtil(branchDAG *BranchDAG) (new *utils) {
	return &utils{
		branchDAG: branchDAG,
	}
}
