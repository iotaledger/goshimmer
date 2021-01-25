package mana

import (
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
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

// FromMetadataObjectStorage unmsarshalls bytes into a metadata.
func FromMetadataObjectStorage(_ []byte, data []byte) (result objectstorage.StorableObject, err error) {
	return parseMetadata(marshalutil.New(data))
}

// CachedConsensusBasePastManaVectorMetadata represents cached ConsensusBasePastVectorMetadata.
type CachedConsensusBasePastManaVectorMetadata struct {
	objectstorage.CachedObject
}

// Retain marks this CachedConsensusBasePastManaVectorMetadata to still be in use by the program.
func (c *CachedConsensusBasePastManaVectorMetadata) Retain() *CachedConsensusBasePastManaVectorMetadata {
	return &CachedConsensusBasePastManaVectorMetadata{c.CachedObject.Retain()}
}

// Consume unwraps the CachedConsensusBasePastManaVectorMetadata and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedConsensusBasePastManaVectorMetadata) Consume(consumer func(pbm *ConsensusBasePastManaVectorMetadata)) bool {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*ConsensusBasePastManaVectorMetadata))
	})
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedConsensusBasePastManaVectorMetadata) Unwrap() *ConsensusBasePastManaVectorMetadata {
	untypedPbm := c.Get()
	if untypedPbm == nil {
		return nil
	}

	typeCastedPbm := untypedPbm.(*ConsensusBasePastManaVectorMetadata)
	if typeCastedPbm == nil || typeCastedPbm.IsDeleted() {
		return nil
	}

	return typeCastedPbm
}

var _ objectstorage.StorableObject = &ConsensusBasePastManaVectorMetadata{}
