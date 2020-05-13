package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/stretchr/testify/assert"
)

func TestMarshalUnmarshal(t *testing.T) {
	originalMetadata := NewPayloadMetadata(payload.GenesisID)

	clonedMetadata, _, err := PayloadMetadataFromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.PayloadID(), clonedMetadata.PayloadID())
	assert.Equal(t, originalMetadata.IsLiked(), clonedMetadata.IsLiked())

	originalMetadata.SetLike(true)

	clonedMetadata, _, err = PayloadMetadataFromBytes(originalMetadata.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalMetadata.PayloadID(), clonedMetadata.PayloadID())
	assert.Equal(t, originalMetadata.IsLiked(), clonedMetadata.IsLiked())
}

func TestPayloadMetadata_SetLike(t *testing.T) {
	originalMetadata := NewPayloadMetadata(payload.GenesisID)

	assert.Equal(t, false, originalMetadata.IsLiked())

	originalMetadata.SetLike(true)

	assert.Equal(t, true, originalMetadata.IsLiked())
}
