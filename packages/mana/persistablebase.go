package mana

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
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

// FromObjectStorage creates an PersistableBaseMana from sequences of key and bytes.
func (p *PersistableBaseMana) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	res, err := p.FromBytes(value)
	copy(res.NodeID[:], key)
	return res, err

}

// FromBytes unmarshals a PersistableBaseMana from a sequence of bytes.
func (p *PersistableBaseMana) FromBytes(data []byte) (*PersistableBaseMana, error) {
	manaVector := new(PersistableBaseMana)
	if p != nil {
		manaVector = p
	}
	_, err := serix.DefaultAPI.Decode(context.Background(), data, manaVector, serix.WithValidation())
	if err != nil {
		err = errors.Wrap(err, "failed to parse PersistableBaseMana")
		return manaVector, err
	}
	return manaVector, err
}
