package mana

import (
	"crypto/sha256"
	"fmt"
	"math"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

// PersistableBaseMana represents a base mana vector that can be persisted.
type PersistableBaseMana struct {
	objectstorage.StorableObjectFlags
	ManaType        Type
	BaseValues      []float64
	EffectiveValues []float64
	LastUpdated     time.Time
	NodeID          identity.ID

	bytes []byte
}

// String returns a human readable version of the PersistableBaseMana.
func (persistableBaseMana *PersistableBaseMana) String() string {
	return stringify.Struct("PersistableBaseMana",
		stringify.StructField("ManaType", fmt.Sprint(persistableBaseMana.ManaType)),
		stringify.StructField("BaseValues", fmt.Sprint(persistableBaseMana.BaseValues)),
		stringify.StructField("EffectiveValues", fmt.Sprint(persistableBaseMana.EffectiveValues)),
		stringify.StructField("LastUpdated", fmt.Sprint(persistableBaseMana.LastUpdated)),
		stringify.StructField("NodeID", persistableBaseMana.NodeID.String()),
	)
}

var _ objectstorage.StorableObject = &PersistableBaseMana{}

// Bytes  marshals the persistable mana into a sequence of bytes.
func (persistableBaseMana *PersistableBaseMana) Bytes() []byte {
	if bytes := persistableBaseMana.bytes; bytes != nil {
		return bytes
	}
	// create marshal helper
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(byte(persistableBaseMana.ManaType))
	marshalUtil.WriteUint16(uint16(len(persistableBaseMana.BaseValues)))
	for _, baseValue := range persistableBaseMana.BaseValues {
		marshalUtil.WriteUint64(math.Float64bits(baseValue))
	}
	marshalUtil.WriteUint16(uint16(len(persistableBaseMana.EffectiveValues)))
	for _, effectiveValue := range persistableBaseMana.EffectiveValues {
		marshalUtil.WriteUint64(math.Float64bits(effectiveValue))
	}
	marshalUtil.WriteTime(persistableBaseMana.LastUpdated)
	marshalUtil.WriteBytes(persistableBaseMana.NodeID.Bytes())

	persistableBaseMana.bytes = marshalUtil.Bytes()
	return persistableBaseMana.bytes
}

// Update updates the persistable mana in storage.
func (persistableBaseMana *PersistableBaseMana) Update(objectstorage.StorableObject) {
	panic("should not be updated")
}

// ObjectStorageKey returns the key of the persistable mana.
func (persistableBaseMana *PersistableBaseMana) ObjectStorageKey() []byte {
	return persistableBaseMana.NodeID.Bytes()
}

// ObjectStorageValue returns the bytes of the persistable mana.
func (persistableBaseMana *PersistableBaseMana) ObjectStorageValue() []byte {
	return persistableBaseMana.Bytes()
}

// FromBytes unmarshals a Persistable Base Mana from a sequence of bytes.
func FromBytes(bytes []byte) (result *PersistableBaseMana, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// Parse unmarshals a persistableBaseMana using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *PersistableBaseMana, err error) {
	result = &PersistableBaseMana{}
	manaType, err := marshalUtil.ReadByte()
	if err != nil {
		return
	}
	result.ManaType = Type(manaType)

	baseValuesLength, err := marshalUtil.ReadUint16()
	if err != nil {
		return
	}
	result.BaseValues = make([]float64, 0, baseValuesLength)
	for i := 0; i < int(baseValuesLength); i++ {
		var baseMana uint64
		baseMana, err = marshalUtil.ReadUint64()
		if err != nil {
			return result, err
		}
		result.BaseValues = append(result.BaseValues, math.Float64frombits(baseMana))
	}

	effectiveValuesLength, err := marshalUtil.ReadUint16()
	if err != nil {
		return result, err
	}
	result.EffectiveValues = make([]float64, 0, effectiveValuesLength)
	for i := 0; i < int(effectiveValuesLength); i++ {
		var effBaseMana uint64
		effBaseMana, err = marshalUtil.ReadUint64()
		if err != nil {
			return result, err
		}
		result.EffectiveValues = append(result.EffectiveValues, math.Float64frombits(effBaseMana))
	}

	lastUpdated, err := marshalUtil.ReadTime()
	if err != nil {
		return
	}
	result.LastUpdated = lastUpdated

	nodeIDBytes, err := marshalUtil.ReadBytes(sha256.Size)
	if err != nil {
		return
	}
	var nodeID identity.ID
	copy(nodeID[:], nodeIDBytes)
	result.NodeID = nodeID

	consumedBytes := marshalUtil.ReadOffset()
	result.bytes = make([]byte, consumedBytes)
	copy(result.bytes, marshalUtil.Bytes())
	return
}

// FromObjectStorage is a factory method that creates a new PersistableBaseMana instance from a storage key of the objectstorage.
func FromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	persistableBaseMana, err := Parse(marshalutil.New(data))
	if err != nil {
		return
	}
	copy(persistableBaseMana.NodeID[:], key)
	result = persistableBaseMana
	return
}

// CachedPersistableBaseMana represents cached persistable mana.
type CachedPersistableBaseMana struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedPbm *CachedPersistableBaseMana) Retain() *CachedPersistableBaseMana {
	return &CachedPersistableBaseMana{cachedPbm.CachedObject.Retain()}
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedPbm *CachedPersistableBaseMana) Consume(consumer func(pbm *PersistableBaseMana)) bool {
	return cachedPbm.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*PersistableBaseMana))
	})
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedPbm *CachedPersistableBaseMana) Unwrap() *PersistableBaseMana {
	untypedPbm := cachedPbm.Get()
	if untypedPbm == nil {
		return nil
	}

	typeCastedPbm := untypedPbm.(*PersistableBaseMana)
	if typeCastedPbm == nil || typeCastedPbm.IsDeleted() {
		return nil
	}

	return typeCastedPbm
}
