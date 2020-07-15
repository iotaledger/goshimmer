package syncbeacon

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPayload(t *testing.T) {
	originalPayload := NewSyncBeaconPayload(true, time.Now().UnixNano())
	clonedPayload1, err, _ := FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.SyncStatus(), clonedPayload1.SyncStatus())

	clonedPayload2, err, _ := FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.SyncStatus(), clonedPayload2.SyncStatus())
}

func TestIsSyncBeaconPayload(t *testing.T) {
	p := NewSyncBeaconPayload(true, time.Now().UnixNano())

	isSyncBeaconPayload := IsSyncBeaconPayload(p)
	assert.True(t, isSyncBeaconPayload)
}
