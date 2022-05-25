package notarization

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

// EI is the ID of an epoch.
type EI uint64

// Epoch is a time range used to define a bucket of messages.
type Epoch struct {
	ei EI

	confirmed     bool
	confirmedTime time.Time
	finalized     bool
	finalizedTime time.Time

	commitment           *EpochCommitment
	commitmentUpdateTime time.Time

	confirmedMutex  sync.RWMutex
	finalizedMutex  sync.RWMutex
	commitmentMutex sync.RWMutex
}

// NewEpoch is the constructor for an Epoch.
func NewEpoch(ei EI) (epoch *Epoch) {
	epoch = &Epoch{
		ei:         ei,
		commitment: &EpochCommitment{},
	}

	return
}

// EI returns the Epoch's EI.
func (e *Epoch) EI() EI {
	return e.ei
}

// Finalized returns true if the epoch is finalized.
func (e *Epoch) Finalized() bool {
	return e.finalized
}

// Confirmed returns true if the epoch is confirmed.
func (e *Epoch) Confirmed() bool {
	return e.confirmed
}

// EC returns the epoch commitment of the epoch.
func (e *Epoch) EC() *EpochCommitment {
	return e.commitment
}

// SetFinalized sets the finalized flag with the given value.
func (e *Epoch) SetFinalized(finalized bool) (modified bool) {
	e.finalizedMutex.RLock()
	if e.finalized != finalized {
		e.finalizedMutex.RUnlock()

		e.finalizedMutex.Lock()
		if e.finalized != finalized {
			e.finalized = finalized
			if finalized {
				e.finalizedTime = clock.SyncedTime()
			}
			modified = true
		}
		e.finalizedMutex.Unlock()
	} else {
		e.finalizedMutex.RUnlock()
	}

	return
}

// SetConfirmed sets the confirmed flag with the given value.
func (e *Epoch) SetConfirmed(confirmed bool) (modified bool) {
	e.confirmedMutex.RLock()
	if e.confirmed != confirmed {
		e.confirmedMutex.RUnlock()

		e.confirmedMutex.Lock()
		if e.confirmed != confirmed {
			e.confirmed = confirmed
			if confirmed {
				e.confirmedTime = clock.SyncedTime()
			}
			modified = true
		}
		e.confirmedMutex.Unlock()
	} else {
		e.confirmedMutex.RUnlock()
	}

	return
}

// SetCommitment sets commitment of the epoch.
func (e *Epoch) SetCommitment(commitment *EpochCommitment) {
	e.commitmentMutex.Lock()
	defer e.commitmentMutex.Unlock()

	e.commitment = commitment
	e.commitmentUpdateTime = clock.SyncedTime()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
