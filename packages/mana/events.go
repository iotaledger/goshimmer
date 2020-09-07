package mana

import (
	"sync"
	"time"

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

// PledgedEvent is the struct that is passed along with triggering a Pledged event.
type PledgedEvent struct {
	NodeID    identity.ID
	AmountBM1 float64
	AmountBM2 float64
	Time      time.Time
	Type      Type // access or consensus
}

// RevokedEvent is the struct that is passed along with triggering a Revoked event.
type RevokedEvent struct {
	NodeID    identity.ID
	AmountBM1 float64
	Time      time.Time
	Type      Type // access or consensus
}

// UpdatedEvent is the struct that is passed along with triggering an Updated event.
type UpdatedEvent struct {
	NodeID  identity.ID
	OldMana BaseMana
	NewMana BaseMana
	Type    Type // access or consensus
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
