package mana

import (
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
)

const (
	// ConsensusBaseManaPastVectorMetadataStorageKey is the key to the consensus base vector metadata storage.
	ConsensusBaseManaPastVectorMetadataStorageKey = "consensusBaseManaVectorMetadata"
)

// ConsensusBasePastManaVectorMetadata holds metadata for the past consensus mana vector.
type ConsensusBasePastManaVectorMetadata struct {
	objectstorage.StorableObjectFlags
	Timestamp time.Time `json:"timestamp"`
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
	c.bytes = marshalUtil.Bytes()
	return c.bytes
}

// ObjectStorageKey returns the key of the metadata.
func (c *ConsensusBasePastManaVectorMetadata) ObjectStorageKey() []byte {
	return []byte(ConsensusBaseManaPastVectorMetadataStorageKey)
}

// ObjectStorageValue returns the bytes of the metadata.
func (c *ConsensusBasePastManaVectorMetadata) ObjectStorageValue() []byte {
	return c.Bytes()
}

// parseMetadata unmarshals a metadata using the given marshalUtil (for easier marshaling/unmarshalling).
func parseMetadata(marshalUtil *marshalutil.MarshalUtil) (result *ConsensusBasePastManaVectorMetadata, err error) {
	timestamp, err := marshalUtil.ReadTime()
	if err != nil {
		return
	}
	consumedBytes := marshalUtil.ReadOffset()
	result = &ConsensusBasePastManaVectorMetadata{
		Timestamp: timestamp,
	}
	result.bytes = make([]byte, consumedBytes)
	copy(result.bytes, marshalUtil.Bytes())
	return
}

// FromObjectStorage creates an ConsensusBasePastManaVectorMetadata from sequences of key and bytes.
func (c *ConsensusBasePastManaVectorMetadata) FromObjectStorage(_, bytes []byte) (objectstorage.StorableObject, error) {
	return c.FromBytes(bytes)
}

// FromBytes unmarshalls bytes into a metadata.
func (*ConsensusBasePastManaVectorMetadata) FromBytes(data []byte) (result *ConsensusBasePastManaVectorMetadata, err error) {
	return parseMetadata(marshalutil.New(data))
}

var _ objectstorage.StorableObject = new(ConsensusBasePastManaVectorMetadata)
