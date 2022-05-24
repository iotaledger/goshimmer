package mana

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsensusBasePastManaVectorMetadata_ObjectStorageKey(t *testing.T) {
	c := &ConsensusBasePastManaVectorMetadata{}
	key := []byte(ConsensusBaseManaPastVectorMetadataStorageKey)
	assert.Equal(t, key, c.ObjectStorageKey(), "should be equal")
}

func TestConsensusBasePastManaVectorMetadata_ObjectStorageValue(t *testing.T) {
	c := &ConsensusBasePastManaVectorMetadata{}
	val := c.ObjectStorageValue()
	assert.Equal(t, c.Bytes(), val, "should be equal")
}

func TestConsensusBasePastManaVectorMetadata_FromBytes(t *testing.T) {
	timestamp := time.Now()
	c := &ConsensusBasePastManaVectorMetadata{
		Timestamp: timestamp,
	}
	c1, err := new(ConsensusBasePastManaVectorMetadata).FromBytes(c.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, c.Bytes(), c1.Bytes(), "should be equal")
}
