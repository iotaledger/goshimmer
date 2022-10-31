package manamodels

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

func TestPersistableEvent_Bytes(t *testing.T) {
	ev := new(PersistableEvent)
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(ev.Type)
	marshalUtil.WriteByte(byte(ev.ManaType))
	marshalUtil.WriteBytes(lo.PanicOnErr(ev.IssuerID.Bytes()))
	marshalUtil.WriteTime(ev.Time)
	marshalUtil.WriteBytes(lo.PanicOnErr(ev.TransactionID.Bytes()))
	marshalUtil.WriteInt64(ev.Amount)
	marshalUtil.WriteBytes(lo.PanicOnErr(ev.InputID.Bytes()))

	bytes := marshalUtil.Bytes()
	assert.Equal(t, bytes, ev.Bytes(), "should be equal")
}

func TestPersistableEvent_ObjectStorageKey(t *testing.T) {
	ev := new(PersistableEvent)
	key := ev.Bytes()
	assert.Equal(t, key, ev.ObjectStorageKey(), "should be equal")
}

func TestPersistableEvent_ObjectStorageValue(t *testing.T) {
	ev := new(PersistableEvent)
	val := ev.ObjectStorageValue()
	assert.Equal(t, ev.Bytes(), val, "should be equal")
}

func TestPersistableEvent_FromBytes(t *testing.T) {
	ev := &PersistableEvent{
		Type:          0,
		IssuerID:      identity.ID{},
		Amount:        100,
		Time:          time.Now(),
		ManaType:      ConsensusMana,
		TransactionID: utxo.TransactionID{},
	}
	ev1, err := new(PersistableEvent).FromBytes(ev.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, ev.Bytes(), ev1.Bytes(), "should be equal")
}
