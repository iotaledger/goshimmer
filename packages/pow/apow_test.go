package pow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setDefaultParameters() {
	BaseDifficulty = 2
	ApowWindow = 5
	AdaptiveRate = 1
}
func TestAPoWIssuance(t *testing.T) {
	setDefaultParameters()

	mw := MessagesWindow{}

	now := time.Now()

	msg1 := MessageAge{"1", now}

	// testing with empty MessagesWindow, must return Base difficulty
	assert.Equal(t, BaseDifficulty, mw.AdaptiveDifficulty(msg1.Timestamp))

	mw.Append(msg1)
	// must append msg1 and return 1
	assert.Equal(t, 1, len(mw.internalSlice))

	msg2 := MessageAge{"2", now.Add(1 * time.Second)}

	// must return Base difficulty + 1
	assert.Equal(t, BaseDifficulty+1, mw.AdaptiveDifficulty(msg2.Timestamp))

	mw.Append(msg2)
	// must append msg2 and return 2
	assert.Equal(t, 2, len(mw.internalSlice))

	msg3 := MessageAge{"3", now.Add(6 * time.Second)}
	// must return Base difficulty
	assert.Equal(t, BaseDifficulty, mw.AdaptiveDifficulty(now.Add(6*time.Second)))

	mw.Append(msg3)
	// must delete all the previous messages and return 1
	assert.Equal(t, 1, len(mw.internalSlice))
}

func TestAPoWCheck(t *testing.T) {
	setDefaultParameters()

	mw := MessagesWindow{}

	now := time.Now()

	msg1 := MessageAge{"1", now}

	// testing with empty MessagesWindow, must match Base difficulty
	assert.True(t, mw.CheckDifficulty(msg1, BaseDifficulty))

	// must append msg1 and return 1
	assert.Equal(t, 1, len(mw.internalSlice))

	msg2 := MessageAge{"2", now.Add(1 * time.Second)}

	// must match Base difficulty + 1
	assert.False(t, mw.CheckDifficulty(msg2, BaseDifficulty))
	assert.True(t, mw.CheckDifficulty(msg2, BaseDifficulty+1))

	// must append msg2 and return 2
	assert.Equal(t, 2, len(mw.internalSlice))

	msg3 := MessageAge{"3", now.Add(6 * time.Second)}
	// must match Base difficulty
	assert.True(t, mw.CheckDifficulty(msg3, BaseDifficulty))

	// must delete all the previous messages and return 1
	assert.Equal(t, 1, len(mw.internalSlice))
}

func TestAPoWNotInOrder(t *testing.T) {
	setDefaultParameters()

	mw := MessagesWindow{}

	now := time.Now()

	msg1 := MessageAge{"1", now}
	msg2 := MessageAge{"2", now.Add(1 * time.Second)}
	msg3 := MessageAge{"3", now.Add(2 * time.Second)}

	assert.True(t, mw.CheckDifficulty(msg3, BaseDifficulty))
	assert.True(t, mw.CheckDifficulty(msg1, BaseDifficulty))
	assert.True(t, mw.CheckDifficulty(msg2, BaseDifficulty+1))
}
