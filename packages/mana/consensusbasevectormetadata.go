package mana

import (
	"strconv"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

// ConsensusBasePastManaVectorMetadata holds metadata for the past consensus mana vector.
type ConsensusBasePastManaVectorMetadata struct {
	objectstorage.StorableObjectFlags
	Timestamp time.Time
	Index     int64
	bytes     []byte
}

// Bytes marshals the consensus base past mana vector metadata into a sequence of bytes.
func (c *ConsensusBasePastManaVectorMetadata) Bytes() []byte {
	if bytes := c.bytes; bytes != nil {
		return bytes
	}
	// create marshal helper
	marshalUtil := marshalutil.New()
	marshalUtil.WriteTime(c.Timestamp)
	marshalUtil.WriteInt64(c.Index)
	c.bytes = marshalUtil.Bytes()
	return c.bytes
}

// Update updates the metadata in storage.
func (c *ConsensusBasePastManaVectorMetadata) Update(other objectstorage.StorableObject) {
	metadata := other.(*ConsensusBasePastManaVectorMetadata)
	c.Index = metadata.Index
	c.Timestamp = metadata.Timestamp
	c.Persist()
	c.SetModified()
}

// ObjectStorageKey returns the key of the metadata.
func (c *ConsensusBasePastManaVectorMetadata) ObjectStorageKey() []byte {
	return []byte(strconv.FormatInt(c.Index, 10))
}

// ObjectStorageValue returns the bytes of the metadata.
func (c *ConsensusBasePastManaVectorMetadata) ObjectStorageValue() []byte {
	return c.Bytes()
}

// parseMetadata unmarshals a metadata using the given marshalUtil (for easier marshaling/unmarshaling).
func parseMetadata(marshalUtil *marshalutil.MarshalUtil) (result *ConsensusBasePastManaVectorMetadata, err error) {
	timestamp, err := marshalUtil.ReadTime()
	if err != nil {
		return
	}
	eventsIndex, err := marshalUtil.ReadInt64()
	if err != nil {
		return
	}
	consumedBytes := marshalUtil.ReadOffset()
	result = &ConsensusBasePastManaVectorMetadata{
		Timestamp: timestamp,
		Index:     eventsIndex,
	}
	result.bytes = make([]byte, consumedBytes)
	copy(result.bytes, marshalUtil.Bytes())
	return
}

// FromMetadataObjectStorage unmsarshalls bytes into a metadata.
func FromMetadataObjectStorage(_ []byte, data []byte) (result objectstorage.StorableObject, err error) {
	return parseMetadata(marshalutil.New(data))
}

var _ objectstorage.StorableObject = &ConsensusBasePastManaVectorMetadata{}
