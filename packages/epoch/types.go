package epoch

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/serix"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/clock"
)

// EI is the ID of an epoch.
type EI uint64

func (e EI) Bytes() []byte {
	bytes, err := serix.DefaultAPI.Encode(context.Background(), e, serix.WithValidation())
	if err != nil {
		panic(err)
	}

	return bytes
}

type MerkleRoot struct {
	types.Identifier `serix:"0"`
}

type ECR = MerkleRoot
type EC = MerkleRoot

// Epoch is a time range used to define a bucket of messages.
type Epoch struct {
	ei EI

	confirmed     bool
	confirmedTime time.Time
	finalized     bool
	finalizedTime time.Time

	confirmedMutex  sync.RWMutex
	finalizedMutex  sync.RWMutex
	commitmentMutex sync.RWMutex
}

// EpochCommitment contains the ECR and PreviousEC of an epoch.
type EpochCommitment struct {
	EI         EI
	ECR        *ECR
	PreviousEC *EC
}

// NewEpoch is the constructor for an Epoch.
func NewEpoch(ei EI) (epoch *Epoch) {
	epoch = &Epoch{
		ei: ei,
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

type EpochDiffs struct {
	orderedmap.OrderedMap[EI, *EpochDiff] `serix:"0"`
}

type EpochDiff struct {
	model.Storable[EI, epochDiff] `serix:"0"`
}

type epochDiff struct {
	EI      EI           `serix:"0"`
	Created utxo.Outputs `serix:"1"`
	Spent   utxo.Outputs `serix:"2"`
}

func NewEpochDiff(ei EI) *EpochDiff {
	return &EpochDiff{
		model.NewStorable(epochDiff{
			EI: ei,
		}, func(model *epochDiff) EI {
			return model.EI
		}),
	}
}

func (e *EpochDiff) EI() EI {
	e.RLock()
	defer e.RUnlock()

	return e.M.EI
}

func (e *EpochDiff) SetEI(ei EI) {
	e.Lock()
	defer e.Unlock()

	e.M.EI = ei
	e.SetModified()
}

func (e *EpochDiff) Created() *utxo.Outputs {
	e.RLock()
	defer e.RUnlock()

	return &utxo.Outputs{*e.M.Created.OrderedMap.Clone()}
}

func (e *EpochDiff) SetCreated(created utxo.Outputs) {
	e.Lock()
	defer e.Unlock()

	e.M.Created = created
	e.SetModified()
}

func (e *EpochDiff) Spent() *utxo.Outputs {
	e.RLock()
	defer e.RUnlock()

	return &utxo.Outputs{*e.M.Spent.OrderedMap.Clone()}
}

func (e *EpochDiff) SetSpent(spent utxo.Outputs) {
	e.Lock()
	defer e.Unlock()

	e.M.Spent = spent
	e.SetModified()
}
