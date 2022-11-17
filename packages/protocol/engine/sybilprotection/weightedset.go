package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"
)

// WeightedSet is a set of identities with a total weight that is kept in sync with the underlying WeightsVector.
type WeightedSet interface {
	// Add adds the given identity to the set.
	Add(id identity.ID) (added bool)

	// Remove removes the given identity from the set.
	Remove(id identity.ID) (removed bool)

	// Has returns true if the given identity is part of the set.
	Has(id identity.ID) (has bool)

	// ForEach iterates over all identities in the set and calls the given callback for each of them (returning an error
	// aborts the iterations).
	ForEach(callback func(id identity.ID) error) (err error)

	// TotalWeight returns the total weight of all identities in the set.
	TotalWeight() (totalWeight uint64)

	// Dispose severs the connection to the underlying WeightsVector and releases all resources.
	Dispose()
}
