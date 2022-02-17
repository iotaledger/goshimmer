package mana

import (
	"time"

	genericobjectstorage "github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// ConsensusBaseManaPastVectorMetadataStorageKey is the key to the consensus base vector metadata storage.
	ConsensusBaseManaPastVectorMetadataStorageKey = "consensusBaseManaVectorMetadata"
)

// ConsensusBasePastManaVectorMetadata holds metadata for the past consensus mana vector.
type ConsensusBasePastManaVectorMetadata struct {
	genericobjectstorage.StorableObjectFlags
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

// Update updates the metadata in storage.
func (c *ConsensusBasePastManaVectorMetadata) Update(other objectstorage.StorableObject) {
	metadata := other.(*ConsensusBasePastManaVectorMetadata)
	c.Timestamp = metadata.Timestamp
	c.Persist()
	c.SetModified()
}

// ObjectStorageKey returns the key of the metadata.
func (c *ConsensusBasePastManaVectorMetadata) ObjectStorageKey() []byte {
	return []byte(ConsensusBaseManaPastVectorMetadataStorageKey)
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
	consumedBytes := marshalUtil.ReadOffset()
	result = &ConsensusBasePastManaVectorMetadata{
		Timestamp: timestamp,
	}
	result.bytes = make([]byte, consumedBytes)
	copy(result.bytes, marshalUtil.Bytes())
	return
}

// FromBytes unmsarshalls bytes into a metadata.
func (*ConsensusBasePastManaVectorMetadata) FromBytes(data []byte) (result genericobjectstorage.StorableObject, err error) {
	return parseMetadata(marshalutil.New(data))
}

var _ genericobjectstorage.StorableObject = &ConsensusBasePastManaVectorMetadata{}
