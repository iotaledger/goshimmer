package activitytracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

func TestActivityTracker_Update(t *testing.T) {
	tf := votes.NewTestFramework(t)
	tf.CreateValidator("A", validator.WithWeight(1))
	tf.CreateValidator("B", validator.WithWeight(1))
	tf.CreateValidator("C", validator.WithWeight(1))

	validatorSet := validator.NewSet(validator.WithValidatorEventTracking(true))
	activityTracker := New(validatorSet, time.Now, WithActivityWindow(time.Second))

	activityTracker.Update(tf.Validator("A"), time.Now())
	activityTracker.Update(tf.Validator("B"), time.Now())
	activityTracker.Update(tf.Validator("C"), time.Now())

	assert.EqualValues(t, 3, validatorSet.TotalWeight())
	assert.EqualValues(t, 3, validatorSet.Size())

	assert.Eventually(t, func() bool {
		return validatorSet.TotalWeight() == 0 && validatorSet.Size() == 0
	}, time.Second*3, time.Millisecond*10)
}
