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

// String returns a human-readable version of the PersistableBaseMana.
func (p *PersistableBaseMana) String() string {
	return stringify.Struct("PersistableBaseMana",
		stringify.StructField("ManaType", fmt.Sprint(p.ManaType)),
		stringify.StructField("BaseValues", fmt.Sprint(p.BaseValues)),
		stringify.StructField("EffectiveValues", fmt.Sprint(p.EffectiveValues)),
		stringify.StructField("LastUpdated", fmt.Sprint(p.LastUpdated)),
		stringify.StructField("NodeID", p.NodeID.String()),
	)
}

var _ objectstorage.StorableObject = new(PersistableBaseMana)

// Bytes  marshals the persistable mana into a sequence of bytes.
func (p *PersistableBaseMana) Bytes() []byte {
	if bytes := p.bytes; bytes != nil {
		return bytes
	}
	// create marshal helper
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(byte(p.ManaType))
	marshalUtil.WriteUint16(uint16(len(p.BaseValues)))
	for _, baseValue := range p.BaseValues {
		marshalUtil.WriteUint64(math.Float64bits(baseValue))
	}
	marshalUtil.WriteUint16(uint16(len(p.EffectiveValues)))
	for _, effectiveValue := range p.EffectiveValues {
		marshalUtil.WriteUint64(math.Float64bits(effectiveValue))
	}
	marshalUtil.WriteTime(p.LastUpdated)
	marshalUtil.WriteBytes(p.NodeID.Bytes())

	p.bytes = marshalUtil.Bytes()
	return p.bytes
}

// ObjectStorageKey returns the key of the persistable mana.
func (p *PersistableBaseMana) ObjectStorageKey() []byte {
	return p.NodeID.Bytes()
}

// ObjectStorageValue returns the bytes of the persistable mana.
func (p *PersistableBaseMana) ObjectStorageValue() []byte {
	return p.Bytes()
}

// FromObjectStorage creates an PersistableBaseMana from sequences of key and bytes.
func (p *PersistableBaseMana) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	res, err := p.FromBytes(bytes)
	copy(res.NodeID[:], key)
	return res, err

}

// FromBytes unmarshals a Persistable Base Mana from a sequence of bytes.
func (p *PersistableBaseMana) FromBytes(bytes []byte) (result *PersistableBaseMana, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = p.FromMarshalUtil(marshalUtil)
	return
}

// FromMarshalUtil unmarshals a PersistableBaseMana using the given marshalUtil (for easier marshaling/unmarshalling).
func (p *PersistableBaseMana) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (persistableBaseMana *PersistableBaseMana, err error) {
	if persistableBaseMana = p; persistableBaseMana == nil {
		persistableBaseMana = new(PersistableBaseMana)
	}
	manaType, err := marshalUtil.ReadByte()
	if err != nil {
		return
	}
	persistableBaseMana.ManaType = Type(manaType)

	baseValuesLength, err := marshalUtil.ReadUint16()
	if err != nil {
		return
	}
	persistableBaseMana.BaseValues = make([]float64, 0, baseValuesLength)
	for i := 0; i < int(baseValuesLength); i++ {
		var baseMana uint64
		baseMana, err = marshalUtil.ReadUint64()
		if err != nil {
			return persistableBaseMana, err
		}
		persistableBaseMana.BaseValues = append(persistableBaseMana.BaseValues, math.Float64frombits(baseMana))
	}

	effectiveValuesLength, err := marshalUtil.ReadUint16()
	if err != nil {
		return persistableBaseMana, err
	}
	persistableBaseMana.EffectiveValues = make([]float64, 0, effectiveValuesLength)
	for i := 0; i < int(effectiveValuesLength); i++ {
		var effBaseMana uint64
		effBaseMana, err = marshalUtil.ReadUint64()
		if err != nil {
			return persistableBaseMana, err
		}
		persistableBaseMana.EffectiveValues = append(persistableBaseMana.EffectiveValues, math.Float64frombits(effBaseMana))
	}

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

	consumedBytes := marshalUtil.ReadOffset()
	persistableBaseMana.bytes = make([]byte, consumedBytes)
	copy(persistableBaseMana.bytes, marshalUtil.Bytes())
	return
}
