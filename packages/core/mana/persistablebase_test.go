package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/stretchr/testify/assert"
)

func TestPersistableBaseMana_Bytes(t *testing.T) {
	p := newPersistableMana()
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(byte(p.ManaType()))
	marshalUtil.WriteUint16(uint16(len(p.BaseValues())))
	for _, val := range p.BaseValues() {
		marshalUtil.WriteUint64(math.Float64bits(val))
	}
	marshalUtil.WriteUint16(uint16(len(p.EffectiveValues())))
	for _, val := range p.EffectiveValues() {
		marshalUtil.WriteUint64(math.Float64bits(val))
	}

	marshalUtil.WriteTime(p.LastUpdated())

	bytes := marshalUtil.Bytes()
	assert.Equal(t, bytes, lo.PanicOnErr(p.Bytes()), "should be equal")
}

func TestPersistableBaseMana_ObjectStorageKey(t *testing.T) {
	p := newPersistableMana()
	key := p.ObjectStorageKey()
	assert.Equal(t, identity.ID{}.Bytes(), key, "should be equal")
}

func TestPersistableBaseMana_ObjectStorageValue(t *testing.T) {
	p := newPersistableMana()
	val := p.ObjectStorageValue()
	assert.Equal(t, lo.PanicOnErr(p.Bytes()), val, "should be equal")
}

func TestPersistableBaseMana_FromBytes(t *testing.T) {
	p1 := newPersistableMana()
	p2 := new(PersistableBaseMana)
	err := p2.FromBytes(lo.PanicOnErr(p1.Bytes()))
	assert.Nil(t, err, "should not have returned error")
	assert.Equal(t, lo.PanicOnErr(p1.Bytes()), lo.PanicOnErr(p2.Bytes()), "should be equal")
}

func newPersistableMana() *PersistableBaseMana {
	return NewPersistableBaseMana(identity.ID{}, ConsensusMana, []float64{1, 1}, []float64{1, 1}, time.Now())
}
