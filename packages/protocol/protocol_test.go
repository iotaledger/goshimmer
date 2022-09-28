package protocol

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestProtocol(t *testing.T) {
	tf := NewTestFramework(t)

	debug.SetEnabled(true)

	tf.Engine.Tangle.CreateBlock("A", models.WithStrongParents(tf.Engine.Tangle.BlockIDs("Genesis")))
	tf.Engine.Tangle.IssueBlocks("A")
	event.Loop.WaitUntilAllTasksProcessed()

	tf.Engine.Tangle.AssertBooked(map[string]bool{
		"A": true,
	})

}
