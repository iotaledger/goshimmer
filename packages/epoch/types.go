package epoch

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/serix"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

var (
	// GenesisTime is the time (Unix in seconds) of the genesis.
	GenesisTime int64 = 1655985373
	// Duration is the default epoch duration.
	Duration time.Duration = 10 * time.Second
)

// Index is the ID of an epoch.
type Index int64

// IndexFromBytes decodes the given bytes to an Index.
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
	if elapsedSeconds <= 0 {
		return 0
	}

	return Index(elapsedSeconds / int64(Duration))
}

// CurrentIndex returns the EI at the current synced time.
func CurrentIndex() Index {
	return IndexFromTime(clock.SyncedTime())
}

// Bytes returns the bytes of an Index.
func (i Index) Bytes() []byte {
	bytes, err := serix.DefaultAPI.Encode(context.Background(), i, serix.WithValidation())
	if err != nil {
		panic(err)
	}

	return bytes
}

// String returns the string representation of an Index.
func (i Index) String() string {
	return fmt.Sprintf("EI(%d)", i)
}

// StartTime returns the start time of the given epoch.
func (i Index) StartTime() time.Time {
	startUnix := GenesisTime + int64(i)*int64(Duration)
	return time.Unix(startUnix, 0)
}

// EndTime returns the end time of the given epoch.
func (i Index) EndTime() time.Time {
	endUnix := GenesisTime + int64(i)*int64(Duration) + int64(Duration) - 1
	return time.Unix(endUnix, 0)
}

// MerkleRoot represents a Merkle root byte array as a blake2b-256 hash.
type MerkleRoot [blake2b.Size256]byte

// ECR is the ECR of an ECRecord.
type ECR = MerkleRoot

// EC is the EC of an ECRecord.
type EC = MerkleRoot

// NewMerkleRoot creates a new Merkle root from the given bytes.
func NewMerkleRoot(bytes []byte) (mr MerkleRoot) {
	b := [blake2b.Size256]byte{}
	copy(b[:], bytes[:])
	return b
}

// Base58 returns the Base58 representation of the MerkleRoot.
func (m MerkleRoot) Base58() string {
	return base58.Encode(m[:])
}

// Bytes returns the MerkleRoot as a byte slice.
func (m MerkleRoot) Bytes() []byte {
	return m[:]
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

// EI returns the epoch Index of an ECRecord.
func (e *ECRecord) EI() Index {
	e.RLock()
	defer e.RUnlock()

	return e.M.EI
}

// SetEI sets the epoch Index of an ECRecord.
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
