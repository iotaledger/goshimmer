package mana

import (
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

var (
	once       sync.Once
	manaEvents *EventDefinitions
)

func newEvents() *EventDefinitions {
	return &EventDefinitions{
		Pledged: events.NewEvent(pledgeEventCaller),
		Revoked: events.NewEvent(revokedEventCaller),
		Updated: events.NewEvent(updatedEventCaller),
	}
}

// Events returns the events defined in the package.
func Events() *EventDefinitions {
	once.Do(func() {
		manaEvents = newEvents()
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
	// Type returns the type of the event.
	Type() byte
	// ToJSONSerializable returns a struct that can be serialized into JSON object.
	ToJSONSerializable() interface{}
	// ToPersistable returns an event that can be persisted.
	ToPersistable() *PersistableEvent
	// Timestamp returns the time of the event.
	Timestamp() time.Time
	// String returns a human readable version of the event.
	String() string
}

// PledgedEvent is the struct that is passed along with triggering a Pledged event.
type PledgedEvent struct {
	NodeID        identity.ID
	Amount        float64
	Time          time.Time
	ManaType      Type // access or consensus
	TransactionID ledgerstate.TransactionID
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
		TxID:     p.TransactionID.Base58(),
		Amount:   p.Amount,
	}
}

// String returns a human readable version of the event.
func (p *PledgedEvent) String() string {
	return stringify.Struct("PledgeEvent",
		stringify.StructField("type", p.ManaType.String()),
		stringify.StructField("shortNodeID", p.NodeID.String()),
		stringify.StructField("fullNodeID", base58.Encode(p.NodeID.Bytes())),
		stringify.StructField("time", p.Time.String()),
		stringify.StructField("amount", p.Amount),
		stringify.StructField("txID", p.TransactionID),
	)
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
			InputID:       p.InputID,
		}
		return revokeEvent, nil
	}
	return nil, ErrUnknownManaEvent
}

// Type returns the type of the event.
func (p *PledgedEvent) Type() byte {
	return EventTypePledge
}

// Timestamp returns time the event was fired.
func (p *PledgedEvent) Timestamp() time.Time {
	return p.Time
}

var _ Event = &PledgedEvent{}

// RevokedEvent is the struct that is passed along with triggering a Revoked event.
type RevokedEvent struct {
	NodeID        identity.ID
	Amount        float64
	Time          time.Time
	ManaType      Type // shall only be consensus for now
	TransactionID ledgerstate.TransactionID
	InputID       ledgerstate.OutputID
}

// RevokedEventJSON is a JSON serializable form of a RevokedEvent.
type RevokedEventJSON struct {
	ManaType string  `json:"manaType"`
	NodeID   string  `json:"nodeID"`
	Time     int64   `json:"time"`
	TxID     string  `json:"txID"`
	Amount   float64 `json:"amount"`
	InputID  string  `json:"inputID"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (r *RevokedEvent) ToJSONSerializable() interface{} {
	return &RevokedEventJSON{
		ManaType: r.ManaType.String(),
		NodeID:   r.NodeID.String(),
		Time:     r.Time.Unix(),
		TxID:     r.TransactionID.Base58(),
		Amount:   r.Amount,
		InputID:  r.InputID.Base58(),
	}
}

// String returns a human readable version of the event.
func (r *RevokedEvent) String() string {
	return stringify.Struct("RevokedEvent",
		stringify.StructField("type", r.ManaType.String()),
		stringify.StructField("shortNodeID", r.NodeID.String()),
		stringify.StructField("fullNodeID", base58.Encode(r.NodeID.Bytes())),
		stringify.StructField("time", r.Time.String()),
		stringify.StructField("amount", r.Amount),
		stringify.StructField("txID", r.TransactionID),
		stringify.StructField("inputID", r.InputID),
	)
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
		InputID:       r.InputID,
	}
}

// Type returns the type of the event.
func (r *RevokedEvent) Type() byte {
	return EventTypeRevoke
}

// Timestamp returns time the event was fired.
func (r *RevokedEvent) Timestamp() time.Time {
	return r.Time
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

// String returns a human readable version of the event.
func (u *UpdatedEvent) String() string {
	return stringify.Struct("UpdatedEvent",
		stringify.StructField("type", u.ManaType.String()),
		stringify.StructField("shortNodeID", u.NodeID.String()),
		stringify.StructField("fullNodeID", base58.Encode(u.NodeID.Bytes())),
		stringify.StructField("oldBaseMana", u.OldMana),
		stringify.StructField("newBaseMana", u.NewMana),
	)
}

// ToPersistable converts the event to a persistable event.
func (u *UpdatedEvent) ToPersistable() *PersistableEvent {
	panic("cannot persist update event")
}

// Type returns the type of the event.
func (u *UpdatedEvent) Type() byte {
	return EventTypeUpdate
}

// Timestamp returns time the event was fired.
func (u *UpdatedEvent) Timestamp() time.Time {
	panic("not implemented")
}

var _ Event = &UpdatedEvent{}

// EventSlice is a slice of events.
type EventSlice []Event

// Sort sorts a slice of events ASC by their timestamp with preference for RevokedEvent.
func (e EventSlice) Sort() {
	sort.Slice(e, func(i, j int) bool {
		var timeI, timeJ time.Time
		var typeI, _ byte
		switch e[i].Type() {
		case EventTypePledge:
			timeI = e[i].(*PledgedEvent).Time
			typeI = EventTypePledge
		case EventTypeRevoke:
			timeI = e[i].(*RevokedEvent).Time
			typeI = EventTypeRevoke
		}

		switch e[j].Type() {
		case EventTypePledge:
			timeJ = e[j].(*PledgedEvent).Time
			_ = EventTypePledge
		case EventTypeRevoke:
			timeJ = e[j].(*RevokedEvent).Time
			_ = EventTypeRevoke
		}

		if !timeI.Equal(timeJ) {
			return timeI.Before(timeJ)
		}

		return typeI == EventTypeRevoke
	})
}

func pledgeEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *PledgedEvent))(params[0].(*PledgedEvent))
}

func revokedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *RevokedEvent))(params[0].(*RevokedEvent))
}

func updatedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *UpdatedEvent))(params[0].(*UpdatedEvent))
}
