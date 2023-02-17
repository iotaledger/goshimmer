package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/workerpool"
)

func Test(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	tf.BlockDAG.CreateBlock("block1")
	tf.BlockDAG.CreateBlock("block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("block1")))
	tf.BlockDAG.IssueBlocks("block1", "block2")

	tf.BlockDAG.AssertSolid(map[string]bool{
		"block1": true,
		"block2": true,
	})
}
