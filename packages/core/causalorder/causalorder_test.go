package causalorder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// TestCausalOrder_Queue tests the queueing of entities in the CausalOrder.
func TestCausalOrder_Queue(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers)

	tf.CreateEntity("A", WithParents(tf.EntityIDs("Genesis")), WithSlot(1))
	tf.CreateEntity("B", WithParents(tf.EntityIDs("A")), WithSlot(1))
	tf.CreateEntity("C", WithParents(tf.EntityIDs("A", "B")), WithSlot(1))
	tf.CreateEntity("D", WithParents(tf.EntityIDs("C", "B")), WithSlot(1))
	tf.CreateEntity("E", WithParents(tf.EntityIDs("C", "D")), WithSlot(1))

	tf.Queue(tf.Entity("A"))
	workers.WaitChildren()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("A"))
	workers.WaitChildren()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("D"))
	workers.WaitChildren()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("E"))
	workers.WaitChildren()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("C"))
	workers.WaitChildren()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("B"))
	workers.WaitChildren()
	tf.AssertOrdered("A", "B", "C", "D", "E")
}

// TestCausalOrder_EvictSlot tests the eviction of entities in the CausalOrder.
func TestCausalOrder_EvictSlot(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers)
	tf.CreateEntity("A", WithParents(tf.EntityIDs("Genesis")), WithSlot(1))
	tf.CreateEntity("B", WithParents(tf.EntityIDs("A")), WithSlot(1))
	tf.CreateEntity("C", WithParents(tf.EntityIDs("A", "B")), WithSlot(1))
	tf.CreateEntity("D", WithParents(tf.EntityIDs("C", "B")), WithSlot(1))
	tf.CreateEntity("E", WithParents(tf.EntityIDs("C", "D")), WithSlot(1))
	tf.CreateEntity("F", WithParents(tf.EntityIDs("Genesis")), WithSlot(1))
	tf.CreateEntity("G", WithParents(tf.EntityIDs("F")), WithSlot(1))
	tf.CreateEntity("H", WithParents(tf.EntityIDs("G")), WithSlot(2))

	tf.Queue(tf.Entity("A"))
	workers.WaitChildren()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("D"))
	workers.WaitChildren()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("E"))
	workers.WaitChildren()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("C"))
	workers.WaitChildren()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("B"))
	workers.WaitChildren()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("G"))
	workers.WaitChildren()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted()

	tf.EvictSlot(1)
	workers.WaitChildren()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted("F", "G")

	tf.Queue(tf.Entity("F"))
	workers.WaitChildren()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted("F", "G")

	tf.Queue(tf.Entity("H"))
	workers.WaitChildren()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted("F", "G", "H")
}

// TestCausalOrder_UnexpectedCases tests the unexpected cases of the CausalOrder.
func TestCausalOrder_UnexpectedCases(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers)
	tf.CreateEntity("A", WithParents(tf.EntityIDs("Genesis")), WithSlot(1))
	tf.CreateEntity("B", WithParents(tf.EntityIDs("A")), WithSlot(1))
	tf.CreateEntity("C", WithParents(tf.EntityIDs("A")), WithSlot(1))
	tf.Queue(tf.Entity("C"))

	// test queueing an entity with non-existing parents
	tf.RemoveEntity("A")
	tf.Queue(tf.Entity("B"))
	workers.WaitChildren()
	tf.AssertOrdered()
	tf.AssertEvicted("B")

	// test eviction of non-existing entity
	tf.RemoveEntity("C")
	defer func() {
		require.NotNil(t, recover())
		workers.WaitChildren()
		tf.AssertOrdered()
		tf.AssertEvicted("B")
	}()
	tf.EvictSlot(1)
}
