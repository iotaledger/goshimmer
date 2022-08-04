package booker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

func Test(t *testing.T) {
	tf := NewTestFramework(t)
	// defer tf.Shutdown()
	tf.ledgerTf.CreateTransaction("tx1", 4, "Genesis")
	tf.ledgerTf.CreateTransaction("tx2", 4, "tx1.0")
	tf.ledgerTf.CreateTransaction("tx3", 4, "tx2.2")
	tf.ledgerTf.CreateTransaction("tx4", 4, "tx3.1")

	tf.CreateBlock("block1", models.WithPayload(tf.ledgerTf.Transaction("tx4")))
	tf.CreateBlock("block2", models.WithPayload(tf.ledgerTf.Transaction("tx4")))

	tf.CreateBlock("block3", models.WithPayload(tf.ledgerTf.Transaction("tx3")))

	tf.CreateBlock("block4", models.WithPayload(tf.ledgerTf.Transaction("tx2")))

	tf.CreateBlock("block5", models.WithPayload(tf.ledgerTf.Transaction("tx1")))

	tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block3").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block5").WaitUntilAllTasksProcessed()

	fmt.Println(tf.Booker.Block(tf.Block("block1").ID()))
}
