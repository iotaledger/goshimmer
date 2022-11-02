package manamodels

import (
	"time"

	"github.com/iotaledger/hive.go/core/identity"
)

// BaseMana is an interface for a collection of base mana values of a single node.
type BaseMana interface {
	revoke(int64) error
	pledge(int64)
	BaseValue() int64
}

// ManaRetrievalFunc returns the mana value of a node with default weights.
type ManaRetrievalFunc func(identity.ID, ...time.Time) (int64, time.Time, error)
