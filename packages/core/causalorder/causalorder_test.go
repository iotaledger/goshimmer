package causalorder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/workerpool"
)

// TestCausalOrder_Queue tests the queueing of entities in the CausalOrder.
func TestCausalOrder_Queue(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers)

	tf.CreateEntity("A", WithParents(tf.EntityIDs("Genesis")), WithEpoch(1))
	tf.CreateEntity("B", WithParents(tf.EntityIDs("A")), WithEpoch(1))
	tf.CreateEntity("C", WithParents(tf.EntityIDs("A", "B")), WithEpoch(1))
	tf.CreateEntity("D", WithParents(tf.EntityIDs("C", "B")), WithEpoch(1))
	tf.CreateEntity("E", WithParents(tf.EntityIDs("C", "D")), WithEpoch(1))

	tf.Queue(tf.Entity("A"))
	workers.Wait()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("A"))
	workers.Wait()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("D"))
	workers.Wait()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("E"))
	workers.Wait()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("C"))
	workers.Wait()
	tf.AssertOrdered("A")

	tf.Queue(tf.Entity("B"))
	workers.Wait()
	tf.AssertOrdered("A", "B", "C", "D", "E")
}

// TestCausalOrder_EvictEpoch tests the eviction of entities in the CausalOrder.
func TestCausalOrder_EvictEpoch(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers)
	tf.CreateEntity("A", WithParents(tf.EntityIDs("Genesis")), WithEpoch(1))
	tf.CreateEntity("B", WithParents(tf.EntityIDs("A")), WithEpoch(1))
	tf.CreateEntity("C", WithParents(tf.EntityIDs("A", "B")), WithEpoch(1))
	tf.CreateEntity("D", WithParents(tf.EntityIDs("C", "B")), WithEpoch(1))
	tf.CreateEntity("E", WithParents(tf.EntityIDs("C", "D")), WithEpoch(1))
	tf.CreateEntity("F", WithParents(tf.EntityIDs("Genesis")), WithEpoch(1))
	tf.CreateEntity("G", WithParents(tf.EntityIDs("F")), WithEpoch(1))
	tf.CreateEntity("H", WithParents(tf.EntityIDs("G")), WithEpoch(2))

	tf.Queue(tf.Entity("A"))
	workers.Wait()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("D"))
	workers.Wait()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("E"))
	workers.Wait()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("C"))
	workers.Wait()
	tf.AssertOrdered("A")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("B"))
	workers.Wait()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted()

	tf.Queue(tf.Entity("G"))
	workers.Wait()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted()

	tf.EvictEpoch(1)
	workers.Wait()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted("F", "G")

	tf.Queue(tf.Entity("F"))
	workers.Wait()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted("F", "G")

	tf.Queue(tf.Entity("H"))
	workers.Wait()
	tf.AssertOrdered("A", "B", "C", "D", "E")
	tf.AssertEvicted("F", "G", "H")
}

// TestCausalOrder_UnexpectedCases tests the unexpected cases of the CausalOrder.
func TestCausalOrder_UnexpectedCases(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers)
	tf.CreateEntity("A", WithParents(tf.EntityIDs("Genesis")), WithEpoch(1))
	tf.CreateEntity("B", WithParents(tf.EntityIDs("A")), WithEpoch(1))
	tf.CreateEntity("C", WithParents(tf.EntityIDs("A")), WithEpoch(1))
	tf.Queue(tf.Entity("C"))

	// test queueing an entity with non-existing parents
	tf.RemoveEntity("A")
	tf.Queue(tf.Entity("B"))
	workers.Wait()
	tf.AssertOrdered()
	tf.AssertEvicted("B")

	// test eviction of non-existing entity
	tf.RemoveEntity("C")
	defer func() {
		require.NotNil(t, recover())
		workers.Wait()
		tf.AssertOrdered()
		tf.AssertEvicted("B")
	}()
	tf.EvictEpoch(1)
}
