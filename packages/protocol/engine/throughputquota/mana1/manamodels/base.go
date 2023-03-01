package manamodels

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/identity"
)

// ManaRetrievalFunc returns the mana value of a node with default weights.
type ManaRetrievalFunc func(identity.ID, ...time.Time) (int64, time.Time, error)
