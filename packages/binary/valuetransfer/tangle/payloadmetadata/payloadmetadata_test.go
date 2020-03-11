package payloadmetadata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
)

func TestMarshalUnmarshal(t *testing.T) {
	originalMetadata := New(id.Genesis)

	clonedMetadata, err, _ := FromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.GetPayloadId(), clonedMetadata.GetPayloadId())
	assert.Equal(t, originalMetadata.IsSolid(), clonedMetadata.IsSolid())
	assert.Equal(t, originalMetadata.GetSoldificationTime().Round(time.Second), clonedMetadata.GetSoldificationTime().Round(time.Second))

	originalMetadata.SetSolid(true)

	clonedMetadata, err, _ = FromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.GetPayloadId(), clonedMetadata.GetPayloadId())
	assert.Equal(t, originalMetadata.IsSolid(), clonedMetadata.IsSolid())
	assert.Equal(t, originalMetadata.GetSoldificationTime().Round(time.Second), clonedMetadata.GetSoldificationTime().Round(time.Second))
}

func TestPayloadMetadata_SetSolid(t *testing.T) {
	originalMetadata := New(id.Genesis)

	assert.Equal(t, false, originalMetadata.IsSolid())
	assert.Equal(t, time.Time{}, originalMetadata.GetSoldificationTime())

	originalMetadata.SetSolid(true)

	assert.Equal(t, true, originalMetadata.IsSolid())
	assert.Equal(t, time.Now().Round(time.Second), originalMetadata.GetSoldificationTime().Round(time.Second))
}
