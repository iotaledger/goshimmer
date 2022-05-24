package mana

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
)

const (
	// ConsensusBaseManaPastVectorMetadataStorageKey is the key to the consensus base vector metadata storage.
	ConsensusBaseManaPastVectorMetadataStorageKey = "consensusBaseManaVectorMetadata"
)

// ConsensusBasePastManaVectorMetadata holds metadata for the past consensus mana vector.
type ConsensusBasePastManaVectorMetadata struct {
	objectstorage.StorableObjectFlags
	Timestamp time.Time `serix:"0" json:"timestamp"`
	bytes     []byte
}

// Bytes marshals the consensus base past mana vector metadata into a sequence of bytes.
func (c *ConsensusBasePastManaVectorMetadata) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), c, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
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
func (c *ConsensusBasePastManaVectorMetadata) FromObjectStorage(_, value []byte) (objectstorage.StorableObject, error) {
	return c.FromBytes(value)
}

// FromBytes unmarshalls bytes into a metadata.
func (c *ConsensusBasePastManaVectorMetadata) FromBytes(data []byte) (result *ConsensusBasePastManaVectorMetadata, err error) {
	if result = c; result == nil {
		result = new(ConsensusBasePastManaVectorMetadata)
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data, result, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse SigLockedColoredOutput: %w", err)
		return
	}
	return
}

var _ objectstorage.StorableObject = new(ConsensusBasePastManaVectorMetadata)
