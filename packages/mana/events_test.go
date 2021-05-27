package mana

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestRevokedEvent_ToPersistable(t *testing.T) {
	r := newRevokeEvent()
	re := r.ToPersistable()
	assert.Equal(t, r.NodeID, re.NodeID)
	assert.Equal(t, r.Amount, re.Amount)
	assert.Equal(t, r.Time, re.Time)
	assert.Equal(t, r.ManaType, re.ManaType)
	assert.Equal(t, r.TransactionID, re.TransactionID)
}

func TestRevokedEvent_Type(t *testing.T) {
	r := newRevokeEvent()
	assert.Equal(t, EventTypeRevoke, r.Type())
}

func TestRevokeEvent_ToJSONSerializable(t *testing.T) {
	r := newRevokeEvent()
	rj := r.ToJSONSerializable().(*RevokedEventJSON)
	assert.Equal(t, r.NodeID.String(), rj.NodeID)
	assert.Equal(t, r.Amount, rj.Amount)
	assert.Equal(t, r.Time.Unix(), rj.Time)
	assert.Equal(t, r.ManaType.String(), rj.ManaType)
	assert.Equal(t, r.TransactionID.Base58(), rj.TxID)
}

func TestPledgedEvent_ToPersistable(t *testing.T) {
	p := newPledgeEvent()
	pe := p.ToPersistable()
	assert.Equal(t, p.NodeID, pe.NodeID)
	assert.Equal(t, p.Amount, pe.Amount)
	assert.Equal(t, p.Time, pe.Time)
	assert.Equal(t, p.ManaType, pe.ManaType)
	assert.Equal(t, p.TransactionID, pe.TransactionID)
}

func TestPledgedEvent_Type(t *testing.T) {
	p := newPledgeEvent()
	assert.Equal(t, EventTypePledge, p.Type())
}

func TestPledgedEvent_ToJSONSerializable(t *testing.T) {
	p := newPledgeEvent()
	pj := p.ToJSONSerializable().(*PledgedEventJSON)
	assert.Equal(t, p.NodeID.String(), pj.NodeID)
	assert.Equal(t, p.Amount, pj.Amount)
	assert.Equal(t, p.Time.Unix(), pj.Time)
	assert.Equal(t, p.ManaType.String(), pj.ManaType)
	assert.Equal(t, p.TransactionID.Base58(), pj.TxID)
}

func TestUpdatedEvent_ToPersistable(t *testing.T) {
	ev := newUpdateEvent()
	assert.Panics(t, func() {
		ev.ToPersistable()
	}, "should have paniced")
}

func newPledgeEvent() *PledgedEvent {
	return &PledgedEvent{
		NodeID:        identity.ID{},
		Amount:        100,
		Time:          time.Now(),
		ManaType:      ConsensusMana,
		TransactionID: randomTxID(),
	}
}

func newRevokeEvent() *RevokedEvent {
	return &RevokedEvent{
		NodeID:        randomNodeID(),
		Amount:        100,
		Time:          time.Now(),
		ManaType:      ConsensusMana,
		TransactionID: randomTxID(),
	}
}

func newUpdateEvent() *UpdatedEvent {
	return &UpdatedEvent{
		NodeID:   randomNodeID(),
		OldMana:  &ConsensusBaseMana{},
		NewMana:  &ConsensusBaseMana{},
		ManaType: ConsensusMana,
	}
}

func randomNodeID() (iID identity.ID) {
	idBytes := make([]byte, sha256.Size)
	_, _ = rand.Read(idBytes)
	copy(iID[:], idBytes)
	return
}

func randomTxID() (txID ledgerstate.TransactionID) {
	txID, _ = ledgerstate.TransactionIDFromRandomness()
	return
}
