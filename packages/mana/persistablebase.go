package mana

import (
	"crypto/sha256"
	"fmt"
	"math"
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
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

// ObjectStorageKey returns the key of the persistable mana.
func (persistableBaseMana *PersistableBaseMana) ObjectStorageKey() []byte {
	return persistableBaseMana.NodeID.Bytes()
}

// ObjectStorageValue returns the bytes of the persistable mana.
func (persistableBaseMana *PersistableBaseMana) ObjectStorageValue() []byte {
	return persistableBaseMana.Bytes()
}

// FromObjectStorage creates an PersistableBaseMana from sequences of key and bytes.
func (persistableBaseMana *PersistableBaseMana) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	res, err := persistableBaseMana.FromBytes(bytes)
	copy(res.(*PersistableBaseMana).NodeID[:], key)
	return res, err

}

// FromBytes unmarshals a Persistable Base Mana from a sequence of bytes.
func (persistableBaseMana *PersistableBaseMana) FromBytes(bytes []byte) (result objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
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
