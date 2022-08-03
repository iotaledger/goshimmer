package booker

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	tf := NewTestFramework(t)
	// defer tf.Shutdown()

	tf.CreateBlock("block1")
	tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()

	fmt.Println(tf.Booker.Block(tf.Block("block1").ID()))
}
