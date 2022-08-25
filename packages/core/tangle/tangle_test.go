package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

func Test(t *testing.T) {
	tf := NewTestFramework(t)

	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))
	tf.IssueBlocks("block1", "block2").WaitUntilAllTasksProcessed()

	tf.AssertSolid(map[string]bool{
		"block1": true,
		"block2": true,
	})
}
