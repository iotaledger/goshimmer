package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"
)

func TestPersistableBaseMana_Bytes(t *testing.T) {
	p := newPersistableMana()
	marshalUtil := marshalutil.New()
	marshalUtil.WriteInt64(int64(p.ManaType))
	marshalUtil.WriteUint64(math.Float64bits(p.BaseMana1))
	marshalUtil.WriteUint64(math.Float64bits(p.EffectiveBaseMana1))
	marshalUtil.WriteUint64(math.Float64bits(p.BaseMana2))
	marshalUtil.WriteUint64(math.Float64bits(p.EffectiveBaseMana2))
	marshalUtil.WriteTime(p.LastUpdated)
	marshalUtil.WriteBytes(p.NodeID.Bytes())

	bytes := marshalUtil.Bytes()
	assert.Equal(t, bytes, p.Bytes(), "should be equal")

}
func TestPersistableBaseMana_ObjectStorageKey(t *testing.T) {
	p := newPersistableMana()
	key := p.ObjectStorageKey()
	assert.Equal(t, identity.ID{}.Bytes(), key, "should be equal")
}

func TestPersistableBaseMana_ObjectStorageValue(t *testing.T) {
	p := newPersistableMana()
	val := p.ObjectStorageValue()
	assert.Equal(t, p.Bytes(), val, "should be equal")
}

func TestPersistableBaseMana_Update(t *testing.T) {
	p := newPersistableMana()
	assert.Panics(t, func() {
		p.Update(nil)
	}, "should have paniced")
}

func TestPersistableBaseMana_UnmarshalObjectStorageValue(t *testing.T) {
	p1 := newPersistableMana()
	p2 := &PersistableBaseMana{}
	_, err := p2.UnmarshalObjectStorageValue(p1.Bytes())
	assert.Nil(t, err, "should not have returned error")
	assert.Equal(t, p1.Bytes(), p2.Bytes(), "should be equal")
}

func TestFromStorageKey(t *testing.T) {
	p := newPersistableMana()
	p1, _, err := FromStorageKey(p.NodeID.Bytes())
	assert.Nil(t, err, "should not have returned error")
	assert.Equal(t, p.NodeID, p1.NodeID, "should be equal")
}

func newPersistableMana() *PersistableBaseMana {
	return &PersistableBaseMana{
		ManaType:           ConsensusMana,
		BaseMana1:          1,
		EffectiveBaseMana1: 1,
		BaseMana2:          1,
		EffectiveBaseMana2: 1,
		LastUpdated:        time.Now(),
		NodeID:             identity.ID{},
	}
}
