package mana

import (
	"time"

	"github.com/iotaledger/hive.go/identity"
)

// BaseMana is an interface for a collection of base mana values of a single node.
type BaseMana interface {
	update(time.Time) error
	revoke(float64) error
	pledge(*TxInfo) float64
	BaseValue() float64
	EffectiveValue() float64
	LastUpdate() time.Time
}

// ManaRetrievalFunc returns the mana value of a node with default weights.
type ManaRetrievalFunc func(identity.ID, ...time.Time) (float64, time.Time, error)
