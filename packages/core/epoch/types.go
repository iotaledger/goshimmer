package epoch

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
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

type NodesActivityLog map[Index]*ActivityLog

// region ActivityLog //////////////////////////////////////////////////////////////////////////////////////////////////

// ActivityLog is a time-based log of node activity. It stores information when a node is active and provides
// functionality to query for certain timeframes.
type ActivityLog struct {
	SetEpochs set.Set[identity.ID] `serix:"0,lengthPrefixType=uint32"`
}

// NewActivityLog is the constructor for ActivityLog.
func NewActivityLog() *ActivityLog {

	a := &ActivityLog{
		SetEpochs: set.New[identity.ID](),
	}

	return a
}

// Add adds a node to the activity log.
func (a *ActivityLog) Add(nodeID identity.ID) (added bool) {
	return a.SetEpochs.Add(nodeID)
}

// Remove removes a node from the activity log.
func (a *ActivityLog) Remove(nodeID identity.ID) (removed bool) {
	return a.SetEpochs.Delete(nodeID)
}

// Active returns true if the provided node was active.
func (a *ActivityLog) Active(nodeID identity.ID) (active bool) {
	if a.SetEpochs.Has(nodeID) {
		return true
	}

	return
}

// String returns a human-readable version of ActivityLog.
func (a *ActivityLog) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("ActivityLog(len=%d, elements=", a.SetEpochs.Size()))
	a.SetEpochs.ForEach(func(nodeID identity.ID) {
		builder.WriteString(fmt.Sprintf("%s, ", nodeID.String()))

	})
	builder.WriteString(")")
	return builder.String()
}

// Clone clones the ActivityLog.
func (a *ActivityLog) Clone() *ActivityLog {
	clone := NewActivityLog()

	a.SetEpochs.ForEach(func(nodeID identity.ID) {
		clone.SetEpochs.Add(nodeID)
	})

	return clone
}

// Encode serializes the object to a byte slice.
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

	a.SetEpochs = set.New[identity.ID]()
	bytesRead, err = serix.DefaultAPI.Decode(context.Background(), data, &a.SetEpochs, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ActivityLog: %w", err)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// SnapshotEpochActivity is the data structure to store node activity for the snapshot.
type SnapshotEpochActivity map[Index]*SnapshotNodeActivity

// NewSnapshotEpochActivity creates a new SnapshotEpochActivity instance.
func NewSnapshotEpochActivity() map[Index]*SnapshotNodeActivity {
	return make(SnapshotEpochActivity)
}

// SnapshotNodeActivity is structure to store nodes activity for an epoch.
type SnapshotNodeActivity struct {
	model.Immutable[SnapshotNodeActivity, *SnapshotNodeActivity, nodeActivityModel] `serix:"0"`
}

// NewSnapshotNodeActivity creates a new SnapshotNodeActivity instance.
func NewSnapshotNodeActivity() *SnapshotNodeActivity {
	return model.NewImmutable[SnapshotNodeActivity](&nodeActivityModel{NodesLog: make(map[identity.ID]uint64)})
}

// nodeActivityModel stores node identities and corresponding accepted block counters indicating how many blocks node issued in a given epoch.
type nodeActivityModel struct {
	NodesLog map[identity.ID]uint64 `serix:"0,lengthPrefixType=uint32"`
}

// NodesLog returns its activity map of nodes.
func (s *SnapshotNodeActivity) NodesLog() map[identity.ID]uint64 {
	return s.M.NodesLog
}

// NodeActivity returns activity counter for a given node.
func (s *SnapshotNodeActivity) NodeActivity(nodeID identity.ID) uint64 {
	return s.M.NodesLog[nodeID]
}

// SetNodeActivity adds a node activity record to the activity log.
func (s *SnapshotNodeActivity) SetNodeActivity(nodeID identity.ID, activity uint64) {
	s.M.NodesLog[nodeID] = activity
}
