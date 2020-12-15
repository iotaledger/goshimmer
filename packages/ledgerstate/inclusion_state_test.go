package ledgerstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInclusionState_MarshalUnmarshal(t *testing.T) {
	restored, consumedBytes, err := InclusionStateFromBytes(Pending.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, 1, consumedBytes)
	assert.Equal(t, Pending, restored)

	restored, consumedBytes, err = InclusionStateFromBytes(Confirmed.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, 1, consumedBytes)
	assert.Equal(t, Confirmed, restored)

	restored, consumedBytes, err = InclusionStateFromBytes(Rejected.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, 1, consumedBytes)
	assert.Equal(t, Rejected, restored)

	_, consumedBytes, err = InclusionStateFromBytes(InclusionState(4).Bytes())
	assert.Error(t, err)
	assert.Equal(t, 1, consumedBytes)
}

func TestInclusionState_String(t *testing.T) {
	assert.Equal(t, "InclusionState(Pending)", Pending.String())
	assert.Equal(t, "InclusionState(Confirmed)", Confirmed.String())
	assert.Equal(t, "InclusionState(Rejected)", Rejected.String())
	assert.Equal(t, "InclusionState(7)", InclusionState(7).String())
}
