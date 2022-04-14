package mana

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/pkg/errors"
)

// PersistableBaseMana represents a base mana vector that can be persisted.
type PersistableBaseMana struct {
	objectstorage.StorableObjectFlags
	ManaType        Type        `serix:"0"`
	BaseValues      []float64   `serix:"1,lengthPrefixType=uint16"`
	EffectiveValues []float64   `serix:"2,lengthPrefixType=uint16"`
	LastUpdated     time.Time   `serix:"3"`
	NodeID          identity.ID `serix:"4"`

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

// Bytes returns a marshaled version of the PersistableBaseMana.
func (p *PersistableBaseMana) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// Bytes  marshals the persistable mana into a sequence of bytes.
func (p *PersistableBaseMana) BytesOld() []byte {
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

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (p *PersistableBaseMana) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p.NodeID, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the PersistableBaseMana into a sequence of bytes that are used as the value part in the
// object storage.
func (p *PersistableBaseMana) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageKey returns the key of the persistable mana.
func (p *PersistableBaseMana) ObjectStorageKeyOld() []byte {
	return p.NodeID.Bytes()
}

// ObjectStorageValue returns the bytes of the persistable mana.
func (p *PersistableBaseMana) ObjectStorageValueOld() []byte {
	return p.Bytes()
}

// FromObjectStorage creates an PersistableBaseMana from sequences of key and bytes.
func (p *PersistableBaseMana) FromObjectStorageNew(key, bytes []byte) (objectstorage.StorableObject, error) {
	manaVector := new(PersistableBaseMana)
	if manaVector != nil {
		manaVector = p
	}
	_, err := serix.DefaultAPI.Decode(context.Background(), bytes, manaVector, serix.WithValidation())
	if err != nil {
		err = errors.Wrap(err, "failed to parse PersistableBaseMana")
		return manaVector, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), key, &manaVector.NodeID, serix.WithValidation())
	if err != nil {
		err = errors.Wrap(err, "failed to parse PersistableBaseMana.NodeID")
		return manaVector, err
	}
	return manaVector, err
}

// FromObjectStorage creates an PersistableBaseMana from sequences of key and bytes.
func (p *PersistableBaseMana) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	// TODO: remove that eventually
	res, err := p.FromBytes(bytes)
	copy(res.NodeID[:], key)
	return res, err

}

// FromBytes unmarshals a PersistableBaseMana from a sequence of bytes.
func (p *PersistableBaseMana) FromBytesNew(bytes []byte) (*PersistableBaseMana, error) {
	manaVector := new(PersistableBaseMana)
	if manaVector != nil {
		manaVector = p
	}
	_, err := serix.DefaultAPI.Decode(context.Background(), bytes, manaVector, serix.WithValidation())
	if err != nil {
		err = errors.Wrap(err, "failed to parse PersistableBaseMana")
		return manaVector, err
	}
	return manaVector, err
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
