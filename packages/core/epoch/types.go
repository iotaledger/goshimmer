package epoch

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/node/clock"
)

var (
	// GenesisTime is the time (Unix in seconds) of the genesis.
	GenesisTime int64 = 1660128716
	// Duration is the default epoch duration in seconds.
	Duration int64 = 10
)

// Index is the ID of an epoch.
type Index int64

func IndexFromBytes(bytes []byte) (ei Index, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, &ei)
	if err != nil {
		panic(err)
	}

	return
}

// IndexFromTime calculates the EI for the given time.
func IndexFromTime(t time.Time) Index {
	elapsedSeconds := t.Unix() - GenesisTime
	if elapsedSeconds < 0 {
		return 0
	}

	return Index(elapsedSeconds/Duration + 1)
}

func (i Index) Bytes() []byte {
	bytes, err := serix.DefaultAPI.Encode(context.Background(), i, serix.WithValidation())
	if err != nil {
		panic(err)
	}

	return bytes
}

func (i Index) String() string {
	return fmt.Sprintf("EI(%d)", i)
}

// StartTime calculates the start time of the given epoch.
func (i Index) StartTime() time.Time {
	startUnix := GenesisTime + int64(i-1)*Duration
	return time.Unix(startUnix, 0)
}

// EndTime calculates the end time of the given epoch.
func (i Index) EndTime() time.Time {
	endUnix := GenesisTime + int64(i-1)*Duration + Duration - 1
	return time.Unix(endUnix, 0)
}

// CurrentEpochIndex returns the EI at the current RATT time.
func CurrentEpochIndex() Index {
	return IndexFromTime(clock.SyncedTime())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type MerkleRoot [blake2b.Size256]byte

type (
	ECR = MerkleRoot
	EC  = MerkleRoot
)

func NewMerkleRoot(bytes []byte) MerkleRoot {
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

// CommitmentRoots contains roots of trees of an epoch.
type CommitmentRoots struct {
	TangleRoot        MerkleRoot `serix:"0"`
	StateMutationRoot MerkleRoot `serix:"1"`
	StateRoot         MerkleRoot `serix:"2"`
	ManaRoot          MerkleRoot `serix:"3"`
}

// ECRecord is a storable object represents the ecRecord of an epoch.
type ECRecord struct {
	model.Storable[Index, ECRecord, *ECRecord, ecRecord] `serix:"0"`
}

type ecRecord struct {
	EI     Index            `serix:"0"`
	ECR    ECR              `serix:"1"`
	PrevEC EC               `serix:"2"`
	Roots  *CommitmentRoots `serix:"3"`
}

// NewECRecord creates and returns a ECRecord of the given EI.
func NewECRecord(ei Index) (new *ECRecord) {
	new = model.NewStorable[Index, ECRecord](&ecRecord{
		EI:     ei,
		ECR:    MerkleRoot{},
		PrevEC: MerkleRoot{},
		Roots:  &CommitmentRoots{},
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

func (e *ECRecord) Bytes() (bytes []byte, err error) {
	bytes, err = e.Storable.Bytes()
	return
}

func (e *ECRecord) FromBytes(bytes []byte) (err error) {
	err = e.Storable.FromBytes(bytes)
	e.SetID(e.EI())

	return
}

// Roots returns the CommitmentRoots of an ECRecord.
func (e *ECRecord) Roots() *CommitmentRoots {
	e.RLock()
	defer e.RUnlock()

	return e.M.Roots
}

// SetRoots sets the CommitmentRoots of an ECRecord.
func (e *ECRecord) SetRoots(roots *CommitmentRoots) {
	e.Lock()
	defer e.Unlock()

	e.M.Roots = roots
	e.SetModified()
}

// ComputeEC calculates the epoch commitment hash from the given ECRecord.
func (e *ECRecord) ComputeEC() (ec EC) {
	ecHash := blake2b.Sum256(byteutils.ConcatBytes(e.EI().Bytes(), e.ECR().Bytes(), e.PrevEC().Bytes()))

	return NewMerkleRoot(ecHash[:])
}

// region hashing functions ////////////////////////////////////////////////////////////////////////////////////////////

// ComputeECR calculates an ECR from the tree roots.
func ComputeECR(tangleRoot, stateMutationRoot, stateRoot, manaRoot MerkleRoot) ECR {
	branch1Hashed := blake2b.Sum256(byteutils.ConcatBytes(tangleRoot.Bytes(), stateMutationRoot.Bytes()))
	branch2Hashed := blake2b.Sum256(byteutils.ConcatBytes(stateRoot.Bytes(), manaRoot.Bytes()))
	rootHashed := blake2b.Sum256(byteutils.ConcatBytes(branch1Hashed[:], branch2Hashed[:]))

	return NewMerkleRoot(rootHashed[:])
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
