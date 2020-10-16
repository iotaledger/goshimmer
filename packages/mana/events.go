package mana

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
)

var (
	once       sync.Once
	manaEvents *EventDefinitions
)

func new() *EventDefinitions {
	return &EventDefinitions{
		Pledged: events.NewEvent(pledgeEventCaller),
		Revoked: events.NewEvent(revokedEventCaller),
		Updated: events.NewEvent(updatedEventCaller),
	}
}

// Events returns the events defined in the package.
func Events() *EventDefinitions {
	once.Do(func() {
		manaEvents = new()
	})
	return manaEvents
}

// EventDefinitions represents events happening in the mana package.
type EventDefinitions struct {
	// Fired when mana was pledged to a node.
	Pledged *events.Event
	// Fired when mana was revoked from a node.
	Revoked *events.Event
	// Fired when mana of a node was updated.
	Updated *events.Event
}

const (
	// EventTypePledge defines the type of a pledge event.
	EventTypePledge byte = iota
	// EventTypeRevoke defines the event type of a revoke event.
	EventTypeRevoke
	// EventTypeUpdate defines the event type of an updated event.
	EventTypeUpdate
)

// Event is the interface definition of an event.
type Event interface {
	// ManaType returns the type of the event.
	Type() byte
	// ToJSONSerializable returns a struct that can be serialized into JSON object.
	ToJSONSerializable() interface{}
}

// PledgedEvent is the struct that is passed along with triggering a Pledged event.
type PledgedEvent struct {
	NodeID        identity.ID
	AmountBM1     float64
	AmountBM2     float64
	Time          time.Time
	ManaType      Type // access or consensus
	TransactionID transaction.ID
}

// PledgedEventJSON is a JSON serializable form of a PledgedEvent.
type PledgedEventJSON struct {
	ManaType string  `json:"manaType"`
	NodeID   string  `json:"nodeID"`
	Time     int64   `json:"time"`
	TxID     string  `json:"txID"`
	BM1      float64 `json:"bm1"`
	BM2      float64 `json:"bm2"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (p *PledgedEvent) ToJSONSerializable() interface{} {
	return &PledgedEventJSON{
		ManaType: p.ManaType.String(),
		NodeID:   p.NodeID.String(),
		Time:     p.Time.Unix(),
		TxID:     p.TransactionID.String(),
		BM1:      p.AmountBM1,
		BM2:      p.AmountBM2,
	}
}

// Type returns the type of the event.
func (p *PledgedEvent) Type() byte {
	return EventTypePledge
}

var _ Event = &PledgedEvent{}

// RevokedEvent is the struct that is passed along with triggering a Revoked event.
type RevokedEvent struct {
	NodeID        identity.ID
	AmountBM1     float64
	Time          time.Time
	ManaType      Type // access or consensus
	TransactionID transaction.ID
}

// RevokedEventJSON is a JSON serializable form of a RevokedEvent.
type RevokedEventJSON struct {
	ManaType string  `json:"manaType"`
	NodeID   string  `json:"nodeID"`
	Time     int64   `json:"time"`
	TxID     string  `json:"txID"`
	BM1      float64 `json:"bm1"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (r *RevokedEvent) ToJSONSerializable() interface{} {
	return &RevokedEventJSON{
		ManaType: r.ManaType.String(),
		NodeID:   r.NodeID.String(),
		Time:     r.Time.Unix(),
		TxID:     r.TransactionID.String(),
		BM1:      r.AmountBM1,
	}
}

// Type returns the type of the event.
func (r *RevokedEvent) Type() byte {
	return EventTypeRevoke
}

var _ Event = &RevokedEvent{}

// UpdatedEvent is the struct that is passed along with triggering an Updated event.
type UpdatedEvent struct {
	NodeID   identity.ID
	OldMana  BaseMana
	NewMana  BaseMana
	ManaType Type // access or consensus
}

// UpdatedEventJSON is a JSON serializable form of an UpdatedEvent.
type UpdatedEventJSON struct {
	NodeID   string       `json:"nodeID"`
	OldMana  BaseManaJSON `json:"oldMana"`
	NewMana  BaseManaJSON `json:"newMana"`
	ManaType string       `json:"manaType"`
}

// BaseManaJSON is a JSON serializable form of a BaseMana.
type BaseManaJSON struct {
	BaseMana1          float64 `json:"baseMana1"`
	EffectiveBaseMana1 float64 `json:"effectiveBaseMana1"`
	BaseMana2          float64 `json:"baseMana2"`
	EffectiveBaseMana2 float64 `json:"effectiveBaseMana2"`
	LastUpdated        int64   `json:"lastUpdated"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (u *UpdatedEvent) ToJSONSerializable() interface{} {
	return &UpdatedEventJSON{
		ManaType: u.ManaType.String(),
		NodeID:   u.NodeID.String(),
		OldMana: BaseManaJSON{
			BaseMana1:          u.OldMana.BaseMana1,
			EffectiveBaseMana1: u.OldMana.EffectiveBaseMana1,
			BaseMana2:          u.OldMana.BaseMana2,
			EffectiveBaseMana2: u.OldMana.EffectiveBaseMana2,
			LastUpdated:        u.OldMana.LastUpdated.Unix(),
		},
		NewMana: BaseManaJSON{
			BaseMana1:          u.NewMana.BaseMana1,
			EffectiveBaseMana1: u.NewMana.EffectiveBaseMana1,
			BaseMana2:          u.NewMana.BaseMana2,
			EffectiveBaseMana2: u.NewMana.EffectiveBaseMana2,
			LastUpdated:        u.NewMana.LastUpdated.Unix(),
		},
	}
}

// Type returns the type of the event.
func (u *UpdatedEvent) Type() byte {
	return EventTypeUpdate
}

var _ Event = &UpdatedEvent{}

func pledgeEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *PledgedEvent))(params[0].(*PledgedEvent))
}

func revokedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *RevokedEvent))(params[0].(*RevokedEvent))
}
func updatedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *UpdatedEvent))(params[0].(*UpdatedEvent))
}
