package mana

import (
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"
)

func TestConsensusBasePastManaVectorMetadata_Bytes(t *testing.T) {
	c := &ConsensusBasePastManaVectorMetadata{}
	marshalUtil := marshalutil.New()
	marshalUtil.WriteTime(c.Timestamp)
	marshalUtil.WriteInt64(c.Index)
	bytes := marshalUtil.Bytes()
	assert.Equal(t, bytes, c.Bytes(), "should be equal")
}

func TestConsensusBasePastManaVectorMetadata_ObjectStorageKey(t *testing.T) {
	c := &ConsensusBasePastManaVectorMetadata{}
	key := []byte(strconv.FormatInt(c.Index, 10))
	assert.Equal(t, key, c.ObjectStorageKey(), "should be equal")
}

func TestConsensusBasePastManaVectorMetadata_ObjectStorageValue(t *testing.T) {
	c := &ConsensusBasePastManaVectorMetadata{}
	val := c.ObjectStorageValue()
	assert.Equal(t, c.Bytes(), val, "should be equal")
}

func TestConsensusBasePastManaVectorMetadata_Update(t *testing.T) {
	timestamp := time.Now()
	c := &ConsensusBasePastManaVectorMetadata{}
	c1 := &ConsensusBasePastManaVectorMetadata{
		Timestamp: timestamp,
		Index:     10,
	}
	c.Update(c1)
	assert.Equal(t, timestamp, c.Timestamp)
	assert.Equal(t, int64(10), c.Index)
}

func TestFromMetadataObjectStorage(t *testing.T) {
	timestamp := time.Now()
	c := &ConsensusBasePastManaVectorMetadata{
		Timestamp: timestamp,
		Index:     10,
	}
	res, err := FromMetadataObjectStorage([]byte{}, c.Bytes())
	assert.NoError(t, err)
	c1 := res.(*ConsensusBasePastManaVectorMetadata)
	assert.Equal(t, c.Bytes(), c1.Bytes(), "should be equal")
}
