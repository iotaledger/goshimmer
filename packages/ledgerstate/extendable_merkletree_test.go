package ledgerstate

import (
	"testing"
)

func Test(t *testing.T) {
	tree := &ExtendableMerkleTree{
		fullNodesStack: make([]*ExtendableMerkleTreeNode, 0),
	}

	tree.AddLeaf("A")
	tree.AddLeaf("B")
	tree.AddLeaf("C")
	tree.AddLeaf("D")
	tree.AddLeaf("E")
	tree.AddLeaf("F")
	tree.AddLeaf("G")
}
