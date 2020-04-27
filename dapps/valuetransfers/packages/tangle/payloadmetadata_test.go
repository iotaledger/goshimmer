package tangle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
)

func TestMarshalUnmarshal(t *testing.T) {
	originalMetadata := NewPayloadMetadata(payload.GenesisId)

	clonedMetadata, err, _ := PayloadMetadataFromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.GetPayloadId(), clonedMetadata.GetPayloadId())
	assert.Equal(t, originalMetadata.IsSolid(), clonedMetadata.IsSolid())
	assert.Equal(t, originalMetadata.GetSoldificationTime().Round(time.Second), clonedMetadata.GetSoldificationTime().Round(time.Second))

	originalMetadata.SetSolid(true)

	clonedMetadata, err, _ = PayloadMetadataFromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.GetPayloadId(), clonedMetadata.GetPayloadId())
	assert.Equal(t, originalMetadata.IsSolid(), clonedMetadata.IsSolid())
	assert.Equal(t, originalMetadata.GetSoldificationTime().Round(time.Second), clonedMetadata.GetSoldificationTime().Round(time.Second))
}

func TestPayloadMetadata_SetSolid(t *testing.T) {
	originalMetadata := NewPayloadMetadata(payload.GenesisId)

	assert.Equal(t, false, originalMetadata.IsSolid())
	assert.Equal(t, time.Time{}, originalMetadata.GetSoldificationTime())

	originalMetadata.SetSolid(true)

	assert.Equal(t, true, originalMetadata.IsSolid())
	assert.Equal(t, time.Now().Round(time.Second), originalMetadata.GetSoldificationTime().Round(time.Second))
}
