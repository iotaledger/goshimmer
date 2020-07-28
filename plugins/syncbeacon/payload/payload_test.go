package payload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPayload(t *testing.T) {
	originalPayload := NewSyncBeaconPayload(time.Now().UnixNano())
	clonedPayload1, _, err := FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.SentTime(), clonedPayload1.SentTime())

	clonedPayload2, _, err := FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.SentTime(), clonedPayload2.SentTime())
}

func TestIsSyncBeaconPayload(t *testing.T) {
	p := NewSyncBeaconPayload(time.Now().UnixNano())

	isSyncBeaconPayload := IsSyncBeaconPayload(p)
	assert.True(t, isSyncBeaconPayload)
}
