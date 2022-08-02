package epoch

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"
	"sort"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/serix"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

var (
	// GenesisTime is the time (Unix in seconds) of the genesis.
	GenesisTime int64 = 1656588336
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type MerkleRoot [blake2b.Size256]byte

type (
	ECR = MerkleRoot
	EC  = MerkleRoot
)

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

func (e *ECRecord) Bytes() (bytes []byte, err error) {
	bytes, err = e.Storable.Bytes()
	return
}

func (e *ECRecord) FromBytes(bytes []byte) (err error) {
	err = e.Storable.FromBytes(bytes)
	e.SetID(e.EI())

	return
}

type NodesActivityLog map[identity.ID]*ActivityLog

// region ActivityLog //////////////////////////////////////////////////////////////////////////////////////////////////

// ActivityLog is a time-based log of node activity. It stores information when a node is active and provides
// functionality to query for certain timeframes.
type ActivityLog struct {
	SetEpochs set.Set[Index] `serix:"0,lengthPrefixType=uint32"`
}

// NewActivityLog is the constructor for ActivityLog.
func NewActivityLog() *ActivityLog {

	a := &ActivityLog{
		SetEpochs: set.New[Index](),
	}

	return a
}

// Add adds a node activity to the log.
func (a *ActivityLog) Add(ei Index) (added bool) {
	return a.SetEpochs.Add(ei)

}

// Remove removes a node activity from the log.
func (a *ActivityLog) Remove(ei Index) (removed bool) {
	return a.SetEpochs.Delete(ei)
}

// Active returns true if the node was active between lower and upper bound.
func (a *ActivityLog) Active(lowerBound, upperBound Index) (active bool) {
	for ei := lowerBound; ei <= upperBound; ei++ {
		if a.SetEpochs.Has(ei) {
			return true
		}
	}

	return
}

// Clean cleans up the log, meaning that old/stale times are deleted.
// If the log ends up empty after cleaning up, empty is set to true.
func (a *ActivityLog) Clean(cutoff Index) (empty bool) {
	// we remove all activity records below lowerBound as we will no longer need it
	a.SetEpochs.ForEach(func(ei Index) {
		if ei < cutoff {
			a.SetEpochs.Delete(ei)
		}
	})
	if a.SetEpochs.Size() == 0 {
		return true
	}
	return
}

// Epochs returns all epochs stored in this ActivityLog.
func (a *ActivityLog) Epochs() (epochs []Index) {
	epochs = make([]Index, 0, a.SetEpochs.Size())

	// insert in order
	a.SetEpochs.ForEach(func(ei Index) {
		idx := sort.Search(len(epochs), func(i int) bool { return epochs[i] >= ei })
		epochs = append(epochs[:idx+1], epochs[idx:]...)
		epochs[idx] = ei
	})

	return
}

// String returns a human-readable version of ActivityLog.
func (a *ActivityLog) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("ActivityLog(len=%d, elements=", a.SetEpochs.Size()))
	ordered := a.Epochs()
	for _, u := range ordered {
		builder.WriteString(fmt.Sprintf("%d, ", u))
	}
	builder.WriteString(")")
	return builder.String()
}

// Clone clones the ActivityLog.
func (a *ActivityLog) Clone() *ActivityLog {
	clone := NewActivityLog()

	a.SetEpochs.ForEach(func(ei Index) {
		clone.SetEpochs.Add(ei)
	})

	return clone
}

// Encode ActivityLog a serialized byte slice of the object.
func (a *ActivityLog) Encode() ([]byte, error) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a.SetEpochs, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes, nil
}

// Decode deserializes bytes into a valid object.
func (a *ActivityLog) Decode(data []byte) (bytesRead int, err error) {

	a.SetEpochs = set.New[Index]()
	bytesRead, err = serix.DefaultAPI.Decode(context.Background(), data, &a.SetEpochs, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ActivityLog: %w", err)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
