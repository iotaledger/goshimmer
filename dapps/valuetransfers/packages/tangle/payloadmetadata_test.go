package tangle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
)

func TestMarshalUnmarshal(t *testing.T) {
	originalMetadata := NewPayloadMetadata(payload.GenesisID)

	clonedMetadata, _, err := PayloadMetadataFromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.PayloadID(), clonedMetadata.PayloadID())
	assert.Equal(t, originalMetadata.IsSolid(), clonedMetadata.IsSolid())
	assert.Equal(t, originalMetadata.SoldificationTime().Round(time.Second), clonedMetadata.SoldificationTime().Round(time.Second))

	originalMetadata.SetSolid(true)

	clonedMetadata, _, err = PayloadMetadataFromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.PayloadID(), clonedMetadata.PayloadID())
	assert.Equal(t, originalMetadata.IsSolid(), clonedMetadata.IsSolid())
	assert.Equal(t, originalMetadata.SoldificationTime().Round(time.Second), clonedMetadata.SoldificationTime().Round(time.Second))
}

func TestPayloadMetadata_SetSolid(t *testing.T) {
	originalMetadata := NewPayloadMetadata(payload.GenesisID)

	assert.Equal(t, false, originalMetadata.IsSolid())
	assert.Equal(t, time.Time{}, originalMetadata.SoldificationTime())

	originalMetadata.SetSolid(true)

	assert.Equal(t, true, originalMetadata.IsSolid())
	assert.Equal(t, time.Now().Round(time.Second), originalMetadata.SoldificationTime().Round(time.Second))
}
