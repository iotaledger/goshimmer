package engine

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"

	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/models"
)

func TestEngine_Solidification(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t)
	tf.CreateBlock("block1", models.WithStrongParents(tf.BlockIDs("Genesis")))
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))
	tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	tf.AssertSolid(map[string]bool{
		"block1": false,
		"block2": false,
	})

	tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()
	tf.AssertSolid(map[string]bool{
		"block1": true,
		"block2": true,
	})
}
