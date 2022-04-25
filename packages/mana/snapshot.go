package mana

import (
	"time"

	"github.com/iotaledger/hive.go/identity"
)

// Snapshot defines a snapshot of the ledger state.
type Snapshot struct {
	AccessManaByNode map[identity.ID]AccessManaRecord
}

// AccessMana defines the info for the aMana snapshot.
type AccessManaRecord struct {
	Value     float64
	Timestamp time.Time
}
