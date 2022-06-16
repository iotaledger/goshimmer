package epoch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/serix"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/clock"
)

// Index is the ID of an epoch.
type Index uint64

func (e Index) Bytes() []byte {
	bytes, err := serix.DefaultAPI.Encode(context.Background(), e, serix.WithValidation())
	if err != nil {
		panic(err)
	}

	return bytes
}

func (e Index) String() string {
	return fmt.Sprintf("EI(%d)", e)
}

func IndexFromBytes(bytes []byte) (ei Index, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, &ei)
	if err != nil {
		panic(err)
	}

	return
}

type MerkleRoot [blake2b.Size256]byte

type ECR = MerkleRoot
type EC = MerkleRoot

func NewMerkleRoot(bytes []byte) (mr MerkleRoot) {
	b := [blake2b.Size256]byte{}
	copy(b[:], bytes[:])
	return b
}

func (m MerkleRoot) Base58() string {
	return base58.Encode(m[:])
}

func (m MerkleRoot) Bytes() []byte {
	return m[:]
}

// Epoch is a time range used to define a bucket of messages.
type Epoch struct {
	ei Index

	confirmed     bool
	confirmedTime time.Time
	finalized     bool
	finalizedTime time.Time

	confirmedMutex  sync.RWMutex
	finalizedMutex  sync.RWMutex
	commitmentMutex sync.RWMutex
}

// NewEpoch is the constructor for an Epoch.
func NewEpoch(ei Index) (epoch *Epoch) {
	epoch = &Epoch{
		ei: ei,
	}

	return
}

// EI returns the Epoch's EI.
func (e *Epoch) EI() Index {
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
	model.Storable[Index, ECRecord, *ECRecord, ecRecord] `serix:"0"`
}

type ecRecord struct {
	EI     Index `serix:"0"`
	ECR    ECR   `serix:"1"`
	PrevEC EC    `serix:"2"`
}

// NewECRecord creates and returns a ECRecord of the given EI.
func NewECRecord(ei Index) (new *ECRecord) {
	new = model.NewStorable[Index, ECRecord](&ecRecord{
		EI:     ei,
		ECR:    MerkleRoot{},
		PrevEC: MerkleRoot{},
	})
	new.SetID(ei)
	return
}

func (e *ECRecord) EI() Index {
	e.RLock()
	defer e.RUnlock()

	return e.M.EI
}

func (e *ECRecord) SetEI(ei Index) {
	e.Lock()
	defer e.Unlock()

	e.M.EI = ei
	e.SetID(ei)

	e.SetModified()
}

// ECR returns the ECR of an ECRecord.
func (e *ECRecord) ECR() ECR {
	e.RLock()
	defer e.RUnlock()

	return e.M.ECR
}

// SetECR sets the ECR of an ECRecord.
func (e *ECRecord) SetECR(ecr ECR) {
	e.Lock()
	defer e.Unlock()

	e.M.ECR = NewMerkleRoot(ecr[:])
	e.SetModified()
}

// PrevEC returns the EC of an ECRecord.
func (e *ECRecord) PrevEC() EC {
	e.RLock()
	defer e.RUnlock()

	return e.M.PrevEC
}

// SetPrevEC sets the PrevEC of an ECRecord.
func (e *ECRecord) SetPrevEC(prevEC EC) {
	e.Lock()
	defer e.Unlock()

	e.M.PrevEC = NewMerkleRoot(prevEC[:])
	e.SetModified()
}
