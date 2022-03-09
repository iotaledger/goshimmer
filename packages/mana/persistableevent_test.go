package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"
)

func TestPersistableEvent_Bytes(t *testing.T) {
	ev := new(PersistableEvent)
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(ev.Type)
	marshalUtil.WriteByte(byte(ev.ManaType))
	marshalUtil.WriteBytes(ev.NodeID.Bytes())
	marshalUtil.WriteTime(ev.Time)
	marshalUtil.WriteBytes(ev.TransactionID.Bytes())
	marshalUtil.WriteUint64(math.Float64bits(ev.Amount))
	marshalUtil.WriteBytes(ev.InputID.Bytes())

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
		Type:          EventTypePledge,
		NodeID:        identity.ID{},
		Amount:        100,
		Time:          time.Now(),
		ManaType:      ConsensusMana,
		TransactionID: ledgerstate.TransactionID{},
	}
	ev1, err := new(PersistableEvent).FromBytes(ev.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, ev.Bytes(), ev1.Bytes(), "should be equal")
}
