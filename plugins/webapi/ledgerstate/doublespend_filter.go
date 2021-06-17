package ledgerstate

import (
	"bytes"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// DoubleSpendFilter keeps a log of recently submitted transactions and their consumed outputs.
type DoubleSpendFilter struct {
	recentMap map[ledgerstate.OutputID]ledgerstate.TransactionID
	addedAt   map[ledgerstate.TransactionID]time.Time
	mutex     sync.RWMutex
}

// NewDoubleSpendFilter creates a new doubleSpendFilter worker.
func NewDoubleSpendFilter() *DoubleSpendFilter {
	return &DoubleSpendFilter{
		recentMap: map[ledgerstate.OutputID]ledgerstate.TransactionID{},
		addedAt:   map[ledgerstate.TransactionID]time.Time{},
	}
}

// Add adds a transaction and it's consumed inputs to the doubleSpendFilter.
func (d *DoubleSpendFilter) Add(tx *ledgerstate.Transaction) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	now := clock.SyncedTime()
	for _, input := range tx.Essence().Inputs() {
		if input.Type() != ledgerstate.UTXOInputType {
			continue
		}
		casted := input.(*ledgerstate.UTXOInput)
		if casted == nil {
			continue
		}
		d.recentMap[casted.ReferencedOutputID()] = tx.ID()
		d.addedAt[tx.ID()] = now
	}
}

// Remove removes all outputs associated to the given transaction ID.
func (d *DoubleSpendFilter) Remove(txID ledgerstate.TransactionID) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if len(d.recentMap) == 0 {
		return
	}
	if _, has := d.addedAt[txID]; !has {
		return
	}
	d.remove(txID)
	d.shrinkMaps()
}

// HasConflict returns if there is a conflicting output in the internal map wrt to the provided inputs (outputIDs).
func (d *DoubleSpendFilter) HasConflict(outputs ledgerstate.Inputs) (bool, ledgerstate.TransactionID) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	for _, input := range outputs {
		if input.Type() != ledgerstate.UTXOInputType {
			continue
		}
		casted := input.(*ledgerstate.UTXOInput)
		if casted == nil {
			continue
		}
		if txID, has := d.recentMap[casted.ReferencedOutputID()]; has {
			return true, txID
		}
	}
	return false, ledgerstate.TransactionID{}
}

// CleanUp removes transactions from the DoubleSpendFilter if they were added more, than 30s ago.
func (d *DoubleSpendFilter) CleanUp() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	// early return if there is nothing to be cleaned up
	if len(d.addedAt) == 0 {
		return
	}
	now := clock.SyncedTime()
	for txID, addedTime := range d.addedAt {
		if now.Sub(addedTime) > DoubleSpendFilterCleanupInterval {
			d.remove(txID)
		}
	}
	d.shrinkMaps()
}

// remove is a non-concurrency safe internal method.
func (d *DoubleSpendFilter) remove(txID ledgerstate.TransactionID) {
	// remove all outputs
	for outputID, storedTxID := range d.recentMap {
		if bytes.Equal(txID.Bytes(), storedTxID.Bytes()) {
			delete(d.recentMap, outputID)
		}
	}
	delete(d.addedAt, txID)
}

// shrinkMaps is a non-concurrency safe internal method.
func (d *DoubleSpendFilter) shrinkMaps() {
	shrunkRecent := map[ledgerstate.OutputID]ledgerstate.TransactionID{}
	shrunkAddedAt := map[ledgerstate.TransactionID]time.Time{}

	for key, value := range d.recentMap {
		shrunkRecent[key] = value
	}
	for key, value := range d.addedAt {
		shrunkAddedAt[key] = value
	}

	d.recentMap = shrunkRecent
	d.addedAt = shrunkAddedAt
}
