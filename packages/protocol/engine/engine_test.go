package engine

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

func TestEngine_Solidification(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t)
	tf.Engine.Solidification.Requester.Events.BlockRequested.Hook(event.NewClosure(func(blockID models.BlockID) {
		fmt.Println("REQUESTED", blockID)
	}))
	tf.Engine.Solidification.Requester.Events.RequestStopped.Hook(event.NewClosure(func(blockID models.BlockID) {
		fmt.Println("REQUEST STOPPED", blockID)
	}))

	tf.Tangle.CreateBlock("block1", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")))
	tf.Tangle.CreateBlock("block2", models.WithStrongParents(tf.Tangle.BlockIDs("block1")))
	tf.Tangle.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	tf.Tangle.AssertSolid(map[string]bool{
		"block1": false,
		"block2": false,
	})

	tf.Tangle.IssueBlocks("block1").WaitUntilAllTasksProcessed()
	tf.Tangle.AssertSolid(map[string]bool{
		"block1": true,
		"block2": true,
	})
}
