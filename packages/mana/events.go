package mana

import (
	"time"

	"github.com/iotaledger/hive.go/events"
)

// Events represents events happening in the mana package.
type Events struct {
	// Fired when mana was pledged to a node.
	Pledged *events.Event
	// Fired when mana was revoked from a node.
	Revoked *events.Event
	// Fired when mana of a node was updated.
	Updated *events.Event
}

// PledgedEvent is the struct that is passed along with triggering a Pledged event.
type PledgedEvent struct {
	NodeID    []byte
	AmountBM1 int
	AmountBM2 int
	Time      time.Time
	Type      Type // access or consensus
}

// RevokedEvent is the struct that is passed along with triggering a Revoked event.
type RevokedEvent struct {
	NodeID    []byte
	AmountBM1 int
	Time      time.Time
	Type      Type // access or consensus
}

// UpdatedEvent is the struct that is passed along with triggering an Updated event.
type UpdatedEvent struct {
	NodeID  []byte
	OldMana BaseMana
	NewMana BaseMana
	Type    Type // access or consensus
}
