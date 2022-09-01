package mana

import (
	"sort"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/stringify"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

const (
	// EventTypePledge defines the type of a pledge event.
	EventTypePledge byte = iota
	// EventTypeRevoke defines the event type of a revoke event.
	EventTypeRevoke
	// EventTypeUpdate defines the event type of an updated event.
	EventTypeUpdate
)

var Events *ManaEvents

// ManaEvents represents events happening in the mana package.
type ManaEvents struct {
	// Fired when mana was pledged to a node.
	Pledged *event.Event[*PledgedEvent]
	// Fired when mana was revoked from a node.
	Revoked *event.Event[*RevokedEvent]
	// Fired when mana of a node was updated.
	Updated *event.Event[*UpdatedEvent]
}

// newEvents returns a the Events used in the mana package.
func newEvents() *ManaEvents {
	return &ManaEvents{
		Pledged: event.New[*PledgedEvent](),
		Revoked: event.New[*RevokedEvent](),
		Updated: event.New[*UpdatedEvent](),
	}
}

func init() {
	Events = newEvents()
}

// Event is the interface definition of an event.
type Event interface {
	// Type returns the type of the event.
	Type() byte
	// ToJSONSerializable returns a struct that can be serialized into JSON object.
	ToJSONSerializable() interface{}
	// ToPersistable returns an event that can be persisted.
	ToPersistable() *PersistableEvent
	// String returns a human readable version of the event.
	String() string
}

// ManaVectorUpdateEvent is a container that acts as a dictionary for the EpochCommittable event related parameters.
type ManaVectorUpdateEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index
	// Spent are outputs that is spent in a transaction.
	Spent []*ledger.OutputWithMetadata
	// Created are the outputs created in a transaction.
	Created []*ledger.OutputWithMetadata
}

// PledgedEvent is the struct that is passed along with triggering a Pledged event.
type PledgedEvent struct {
	NodeID        identity.ID
	Amount        float64
	Time          time.Time
	ManaType      Type // access or consensus
	TransactionID utxo.TransactionID
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
		stringify.NewStructField("type", p.ManaType.String()),
		stringify.NewStructField("shortNodeID", p.NodeID.String()),
		stringify.NewStructField("fullNodeID", base58.Encode(p.NodeID.Bytes())),
		stringify.NewStructField("time", p.Time.String()),
		stringify.NewStructField("amount", p.Amount),
		stringify.NewStructField("txID", p.TransactionID),
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
	TransactionID utxo.TransactionID
	InputID       utxo.OutputID
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
		stringify.NewStructField("type", r.ManaType.String()),
		stringify.NewStructField("shortNodeID", r.NodeID.String()),
		stringify.NewStructField("fullNodeID", base58.Encode(r.NodeID.Bytes())),
		stringify.NewStructField("time", r.Time.String()),
		stringify.NewStructField("amount", r.Amount),
		stringify.NewStructField("txID", r.TransactionID),
		stringify.NewStructField("inputID", r.InputID),
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
	BaseMana float64 `json:"baseMana"`
}

// ToJSONSerializable returns a struct that can be serialized into JSON object.
func (u *UpdatedEvent) ToJSONSerializable() interface{} {
	return &UpdatedEventJSON{
		ManaType: u.ManaType.String(),
		NodeID:   u.NodeID.String(),
		OldMana: &BaseManaJSON{
			BaseMana: u.OldMana.BaseValue(),
		},
		NewMana: &BaseManaJSON{
			BaseMana: u.NewMana.BaseValue(),
		},
	}
}

// String returns a human readable version of the event.
func (u *UpdatedEvent) String() string {
	return stringify.Struct("UpdatedEvent",
		stringify.NewStructField("type", u.ManaType.String()),
		stringify.NewStructField("shortNodeID", u.NodeID.String()),
		stringify.NewStructField("fullNodeID", base58.Encode(u.NodeID.Bytes())),
		stringify.NewStructField("oldBaseMana", u.OldMana),
		stringify.NewStructField("newBaseMana", u.NewMana),
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
