package causalorder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// This test checks if the internal metadata is correct i.e. that children are assigned correctly and that all the flags are correct.
func TestTangle_Queue(t *testing.T) {
	tf := NewTestFramework()
	tf.CreateEntity("A", 0, WithOrdered(true))
	tf.CreateEntity("B", 1, WithParents(tf.EntityIDs("A")))
	tf.CreateEntity("C", 2, WithParents(tf.EntityIDs("A", "B")))
	tf.CreateEntity("D", 3, WithParents(tf.EntityIDs("C", "B")))
	tf.CreateEntity("E", 4, WithParents(tf.EntityIDs("C", "D")))

	lastOrderedID := NewID(-1)

	causalOrder := New[MockEntityID, *MockOrderedEntity](func(id MockEntityID) (entity *MockOrderedEntity, exists bool) {
		entity, exists = tf.Get(id.alias)
		return
	}, func(entity *MockOrderedEntity) (isOrdered bool) {
		return entity.ordered
	}, func(entity *MockOrderedEntity) (err error) {
		entity.ordered = true

		assert.Greater(t, entity.ID().id, lastOrderedID.id)
		lastOrderedID = entity.ID()

		return nil
	}, func(entity *MockOrderedEntity, reason error) {
		assert.Fail(t, "Entity should not be dropped")
	})

	causalOrder.Queue(tf.Entity("A"))
	causalOrder.Queue(tf.Entity("D"))
	causalOrder.Queue(tf.Entity("E"))
	causalOrder.Queue(tf.Entity("C"))
	causalOrder.Queue(tf.Entity("B"))
}
