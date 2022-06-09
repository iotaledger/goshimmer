package epoch

import (
	"context"
	"sync"
	"time"

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

func EIFromBytes(bytes []byte) (ei EI, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, &ei)
	if err != nil {
		panic(err)
	}

	return
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

// ECRecord is a storable object represents the ecRecord of an epoch.
type ECRecord struct {
	model.Storable[EI, ECRecord, *ECRecord, ecRecord] `serix:"0"`
}

type ecRecord struct {
	EI     EI   `serix:"0"`
	ECR    *ECR `serix:"1"`
	PrevEC *EC  `serix:"2"`
}

// NewECRecord creates and returns a ECRecord of the given EI.
func NewECRecord(ei EI) (new *ECRecord) {
	new = model.NewStorable[EI, ECRecord](&ecRecord{
		EI: ei,
	})
	new.SetID(ei)
	return
}

func (e *ECRecord) EI() EI {
	e.RLock()
	defer e.RUnlock()

	return e.M.EI
}

func (e *ECRecord) SetEI(ei EI) {
	e.Lock()
	defer e.Unlock()

	e.M.EI = ei
	e.SetID(ei)

	e.SetModified()
}

// ECR returns the ECR of an ECRecord.
func (e *ECRecord) ECR() *ECR {
	e.RLock()
	defer e.RUnlock()

	return e.M.ECR
}

// SetECR sets the ECR of an ECRecord.
func (e *ECRecord) SetECR(ecr *ECR) {
	e.Lock()
	defer e.Unlock()

	e.M.ECR = ecr
	e.SetModified()
}

// PrevEC returns the EC of an ECRecord.
func (e *ECRecord) PrevEC() *EC {
	e.RLock()
	defer e.RUnlock()

	return e.M.PrevEC
}

// SetPrevEC sets the PrevEC of an ECRecord.
func (e *ECRecord) SetPrevEC(prevEC *EC) {
	e.Lock()
	defer e.Unlock()

	e.M.PrevEC = prevEC
	e.SetModified()
}