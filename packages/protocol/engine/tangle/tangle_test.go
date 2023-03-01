package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/slot"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func Test(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"), slot.NewTimeProvider(slot.WithGenesisUnixTime(time.Now().Unix())))

	tf.BlockDAG.CreateBlock("block1")
	tf.BlockDAG.CreateBlock("block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("block1")))
	tf.BlockDAG.IssueBlocks("block1", "block2")

	tf.BlockDAG.AssertSolid(map[string]bool{
		"block1": true,
		"block2": true,
	})
}
