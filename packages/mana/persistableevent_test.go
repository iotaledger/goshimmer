package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"
)

func TestPersistableEvent_Bytes(t *testing.T) {
	ev := &PersistableEvent{}
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(ev.Type)
	marshalUtil.WriteInt64(int64(ev.ManaType))
	marshalUtil.WriteBytes(ev.NodeID.Bytes())
	marshalUtil.WriteTime(ev.Time)
	marshalUtil.WriteBytes(ev.TransactionID.Bytes())
	marshalUtil.WriteUint64(math.Float64bits(ev.Amount))

	bytes := marshalUtil.Bytes()
	assert.Equal(t, bytes, ev.Bytes(), "should be equal")
}

func TestPersistableEvent_ObjectStorageKey(t *testing.T) {
	ev := &PersistableEvent{}
	key := []byte(ev.TransactionID.String() + ev.Time.String() + ev.NodeID.String())
	assert.Equal(t, key, ev.ObjectStorageKey(), "should be equal")
}

func TestPersistableEvent_ObjectStorageValue(t *testing.T) {
	ev := &PersistableEvent{}
	val := ev.ObjectStorageValue()
	assert.Equal(t, ev.Bytes(), val, "should be equal")
}

func TestPersistableEvent_Update(t *testing.T) {
	ev := &PersistableEvent{}
	assert.Panics(t, func() {
		ev.Update(nil)
	}, "should have paniced")
}

func TestFromEventObjectStorage(t *testing.T) {
	ev := &PersistableEvent{
		Type:          EventTypePledge,
		NodeID:        identity.ID{},
		Amount:        100,
		Time:          time.Now(),
		ManaType:      ConsensusMana,
		TransactionID: transaction.ID{},
	}
	res, err := FromEventObjectStorage([]byte{}, ev.Bytes())
	ev1 := res.(*PersistableEvent)
	assert.NoError(t, err)
	assert.Equal(t, ev.Bytes(), ev1.Bytes(), "should be equal")
}
