package votes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/impl"
)

func TestActiveNodes_Update(t *testing.T) {
	activityNodes := impl.New(time.Now, impl.WithActivityWindow(time.Second))

	tf := NewTestFramework(t, WithActiveNodes(activityNodes))

	tf.CreateValidator("A", validator.WithWeight(1))
	tf.CreateValidator("B", validator.WithWeight(1))
	tf.CreateValidator("C", validator.WithWeight(1))

	activityNodes.Set(tf.Validator("A"), time.Now())
	activityNodes.Set(tf.Validator("B"), time.Now())
	activityNodes.Set(tf.Validator("C"), time.Now())

	assert.EqualValues(t, 3, tf.ActiveNodes.Weight())

	assert.Eventually(t, func() bool {
		return tf.ActiveNodes.Weight() == 0
	}, time.Second*3, time.Millisecond*10)
}
