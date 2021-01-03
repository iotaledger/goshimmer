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
	// ToPersistable returns an event that can be persisted.
	ToPersistable() *PersistableEvent
}

// PledgedEvent is the struct that is passed along with triggering a Pledged event.
type PledgedEvent struct {
	NodeID        identity.ID
	Amount        float64
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
	Amount   float64 `json:"amount"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (p *PledgedEvent) ToJSONSerializable() interface{} {
	return &PledgedEventJSON{
		ManaType: p.ManaType.String(),
		NodeID:   p.NodeID.String(),
		Time:     p.Time.Unix(),
		TxID:     p.TransactionID.String(),
		Amount:   p.Amount,
	}
}

// ToPersistable returns an event that can be persisted.
func (p *PledgedEvent) ToPersistable() *PersistableEvent {
	return &PersistableEvent{
		Type:          p.Type(),
		NodeID:        p.NodeID,
		Amount:        p.Amount,
		Time:          p.Time,
		ManaType:      p.ManaType,
		TransactionID: p.TransactionID,
	}
}

// FromPersistableEvent parses a persistable event to a regular event.
func FromPersistableEvent(p *PersistableEvent) (Event, error) {
	if p.Type == EventTypePledge {
		pledgeEvent := &PledgedEvent{
			NodeID:        p.NodeID,
			Amount:        p.Amount,
			Time:          p.Time,
			ManaType:      p.ManaType,
			TransactionID: p.TransactionID,
		}
		return pledgeEvent, nil
	}
	if p.Type == EventTypeRevoke {
		revokeEvent := &RevokedEvent{
			NodeID:        p.NodeID,
			Amount:        p.Amount,
			Time:          p.Time,
			ManaType:      p.ManaType,
			TransactionID: p.TransactionID,
		}
		return revokeEvent, nil
	}
	return nil, ErrUnknownManaEvent
}

// Type returns the type of the event.
func (p *PledgedEvent) Type() byte {
	return EventTypePledge
}

var _ Event = &PledgedEvent{}

// RevokedEvent is the struct that is passed along with triggering a Revoked event.
type RevokedEvent struct {
	NodeID        identity.ID
	Amount        float64
	Time          time.Time
	ManaType      Type // shall only be consensus for now
	TransactionID transaction.ID
}

// RevokedEventJSON is a JSON serializable form of a RevokedEvent.
type RevokedEventJSON struct {
	ManaType string  `json:"manaType"`
	NodeID   string  `json:"nodeID"`
	Time     int64   `json:"time"`
	TxID     string  `json:"txID"`
	Amount   float64 `json:"amount"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (r *RevokedEvent) ToJSONSerializable() interface{} {
	return &RevokedEventJSON{
		ManaType: r.ManaType.String(),
		NodeID:   r.NodeID.String(),
		Time:     r.Time.Unix(),
		TxID:     r.TransactionID.String(),
		Amount:   r.Amount,
	}
}

// ToPersistable returns an event that can be persisted.
func (r *RevokedEvent) ToPersistable() *PersistableEvent {
	return &PersistableEvent{
		Type:          r.Type(),
		NodeID:        r.NodeID,
		Amount:        r.Amount,
		Time:          r.Time,
		ManaType:      r.ManaType,
		TransactionID: r.TransactionID,
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
	NodeID   string      `json:"nodeID"`
	OldMana  interface{} `json:"oldMana"`
	NewMana  interface{} `json:"newMana"`
	ManaType string      `json:"manaType"`
}

// BaseManaJSON is a JSON serializable form of a BaseMana.
type BaseManaJSON struct {
	BaseMana          float64 `json:"baseMana"`
	EffectiveBaseMana float64 `json:"effectiveBaseMana"`
	LastUpdated       int64   `json:"lastUpdated"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (u *UpdatedEvent) ToJSONSerializable() interface{} {
	return &UpdatedEventJSON{
		ManaType: u.ManaType.String(),
		NodeID:   u.NodeID.String(),
		OldMana: &BaseManaJSON{
			BaseMana:          u.OldMana.BaseValue(),
			EffectiveBaseMana: u.OldMana.EffectiveValue(),
			LastUpdated:       u.OldMana.LastUpdate().Unix(),
		},
		NewMana: &BaseManaJSON{
			BaseMana:          u.NewMana.BaseValue(),
			EffectiveBaseMana: u.NewMana.EffectiveValue(),
			LastUpdated:       u.NewMana.LastUpdate().Unix(),
		},
	}
}

// ToPersistable converts the event to a persistable event.
func (u *UpdatedEvent) ToPersistable() *PersistableEvent {
	panic("cannot persist update event")
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
