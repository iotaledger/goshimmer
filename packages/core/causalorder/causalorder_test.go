package causalorder

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/event"
)

func TestCausalOrder_Queue(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateEntity("A", 1, WithParents(tf.EntityIDs("Genesis")), WithEpoch(1))
	tf.CreateEntity("B", 2, WithParents(tf.EntityIDs("A")), WithEpoch(1))
	tf.CreateEntity("C", 3, WithParents(tf.EntityIDs("A", "B")), WithEpoch(1))
	tf.CreateEntity("D", 4, WithParents(tf.EntityIDs("C", "B")), WithEpoch(1))
	tf.CreateEntity("E", 5, WithParents(tf.EntityIDs("C", "D")), WithEpoch(1))

	tf.Queue(tf.Entity("A"))
	event.Loop.WaitUntilAllTasksProcessed()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("D"))
	event.Loop.WaitUntilAllTasksProcessed()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("E"))
	event.Loop.WaitUntilAllTasksProcessed()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("C"))
	event.Loop.WaitUntilAllTasksProcessed()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("B"))
	event.Loop.WaitUntilAllTasksProcessed()
	tf.AssertOrdered("A", "B", "C", "D", "E")
}

func TestCausalOrder_EvictEpoch(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateEntity("A", 0, WithParents(tf.EntityIDs("Genesis")), WithEpoch(1))
	tf.CreateEntity("B", 1, WithParents(tf.EntityIDs("A")), WithEpoch(1))
	tf.CreateEntity("C", 2, WithParents(tf.EntityIDs("A", "B")), WithEpoch(1))
	tf.CreateEntity("D", 3, WithParents(tf.EntityIDs("C", "B")), WithEpoch(1))
	tf.CreateEntity("E", 4, WithParents(tf.EntityIDs("C", "D")), WithEpoch(1))

	event.Loop.WaitUntilAllTasksProcessed()
}
