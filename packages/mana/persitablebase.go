package mana

import (
	"crypto/sha256"
	"fmt"
	"math"
	"time"

	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

// PersistableBaseMana represents a base mana vector that can be persisted.
type PersistableBaseMana struct {
	objectstorage.StorableObjectFlags
	ManaType           Type
	BaseMana1          float64
	EffectiveBaseMana1 float64
	BaseMana2          float64
	EffectiveBaseMana2 float64
	LastUpdated        time.Time
	NodeID             identity.ID

	bytes []byte
}

// String returns a human readable version of the PersistableBaseMana.
func (persistableBaseMana *PersistableBaseMana) String() string {
	return stringify.Struct("PersistableBaseMana",
		stringify.StructField("ManaType", fmt.Sprint(persistableBaseMana.ManaType)),
		stringify.StructField("BaseMana1", fmt.Sprint(persistableBaseMana.BaseMana1)),
		stringify.StructField("EffectiveBaseMana1", fmt.Sprint(persistableBaseMana.EffectiveBaseMana1)),
		stringify.StructField("BaseMana2", fmt.Sprint(persistableBaseMana.BaseMana2)),
		stringify.StructField("EffectiveBaseMana2", fmt.Sprint(persistableBaseMana.EffectiveBaseMana2)),
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
	marshalUtil.WriteInt64(int64(persistableBaseMana.ManaType))
	marshalUtil.WriteUint64(math.Float64bits(persistableBaseMana.BaseMana1))
	marshalUtil.WriteUint64(math.Float64bits(persistableBaseMana.EffectiveBaseMana1))
	marshalUtil.WriteUint64(math.Float64bits(persistableBaseMana.BaseMana2))
	marshalUtil.WriteUint64(math.Float64bits(persistableBaseMana.EffectiveBaseMana2))
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

// UnmarshalObjectStorageValue unmarshales the bytes into a persitable mana object.
func (persistableBaseMana *PersistableBaseMana) UnmarshalObjectStorageValue(bytes []byte) (consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)
	manaType, err := marshalUtil.ReadInt64()
	if err != nil {
		return
	}
	persistableBaseMana.ManaType = Type(manaType)

	baseMana1, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	persistableBaseMana.BaseMana1 = math.Float64frombits(baseMana1)

	effBaseMana1, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	persistableBaseMana.EffectiveBaseMana1 = math.Float64frombits(effBaseMana1)

	baseMana2, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	persistableBaseMana.BaseMana2 = math.Float64frombits(baseMana2)

	effBaseMana2, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	persistableBaseMana.EffectiveBaseMana2 = math.Float64frombits(effBaseMana2)

	lastUpdated, err := marshalUtil.ReadTime()
	if err != nil {
		return
	}
	persistableBaseMana.LastUpdated = lastUpdated

	nodeIDBytes, err := marshalUtil.ReadBytes(sha256.Size)
	if err != nil {
		return
	}
	var nodeID identity.ID
	copy(nodeID[:], nodeIDBytes)
	persistableBaseMana.NodeID = nodeID

	consumedBytes = marshalUtil.ReadOffset()
	persistableBaseMana.bytes = make([]byte, consumedBytes)
	copy(persistableBaseMana.bytes, bytes[:consumedBytes])
	return
}

// FromStorageKey returns the persistable mana indexed by the key specified.
func FromStorageKey(key []byte, optionalTargetObject ...*PersistableBaseMana) (result *PersistableBaseMana, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &PersistableBaseMana{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromStorageKey")
	}

	var nodeID identity.ID
	copy(nodeID[:], key)
	result.NodeID = nodeID

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
