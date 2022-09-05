package epoch

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

var (
	// GenesisTime is the time (Unix in seconds) of the genesis.
	GenesisTime int64 = 1662385954
	// Duration is the default epoch duration in seconds.
	Duration int64 = 10
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(nodesActivitySerializableMap{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32))
	if err != nil {
		panic(fmt.Errorf("error registering NodesActivityLog type settings: %w", err))
	}
}

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

// region NodesActivityLog //////////////////////////////////////////////////////////////////////////////////////////////////

type nodesActivitySerializableMap map[Index]*ActivityLog

func (al *nodesActivitySerializableMap) FromBytes(data []byte) (err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, al, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse activeNodes: %w", err)
		return
	}
	return
}

func (al *nodesActivitySerializableMap) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), *al, serix.WithValidation())
	if err != nil {
		panic(err)
	}
	return objBytes
}

func (al *nodesActivitySerializableMap) nodesActivityLog() *NodesActivityLog {
	activity := NewNodesActivityLog()
	for ei, a := range *al {
		activity.Set(ei, a)
	}
	return activity
}

type NodesActivityLog struct {
	shrinkingmap.ShrinkingMap[Index, *ActivityLog] `serix:"0,lengthPrefixType=uint32"`
}

func (al *NodesActivityLog) FromBytes(data []byte) (err error) {
	m := make(nodesActivitySerializableMap)
	err = m.FromBytes(data)
	if err != nil {
		return err
	}
	al.loadActivityLogsMap(m)
	return
}

func (al *NodesActivityLog) Bytes() []byte {
	m := al.activityLogsMap()
	return m.Bytes()
}

func NewNodesActivityLog() *NodesActivityLog {
	return &NodesActivityLog{*shrinkingmap.New[Index, *ActivityLog]()}
}

func (al *NodesActivityLog) activityLogsMap() *nodesActivitySerializableMap {
	activityMap := make(nodesActivitySerializableMap)
	al.ForEach(func(ei Index, activity *ActivityLog) bool {
		activityMap[ei] = activity
		return true
	})
	return &activityMap
}

func (al *NodesActivityLog) loadActivityLogsMap(m nodesActivitySerializableMap) {
	for ei, a := range m {
		al.Set(ei, a)
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ActivityLog //////////////////////////////////////////////////////////////////////////////////////////////////

// ActivityLog is a time-based log of node activity. It stores information when a node is active and provides
// functionality to query for certain timeframes.
type ActivityLog struct {
	model.Mutable[ActivityLog, *ActivityLog, activityLogModel] `serix:"0"`
}

// nodeActivityModel stores node identities and corresponding accepted block counters indicating how many blocks node issued in a given epoch.
type activityLogModel struct {
	ActivityLog *set.AdvancedSet[identity.ID] `serix:"0,lengthPrefixType=uint32"`
}

// NewActivityLog is the constructor for ActivityLog.
func NewActivityLog() *ActivityLog {
	return model.NewMutable[ActivityLog](&activityLogModel{ActivityLog: set.NewAdvancedSet[identity.ID]()})
}

// Add adds a node to the activity log.
func (a *ActivityLog) Add(nodeID identity.ID) (added bool) {
	return a.InnerModel().ActivityLog.Add(nodeID)
}

// Remove removes a node from the activity log.
func (a *ActivityLog) Remove(nodeID identity.ID) (removed bool) {
	return a.InnerModel().ActivityLog.Delete(nodeID)
}

// Active returns true if the provided node was active.
func (a *ActivityLog) Active(nodeID identity.ID) (active bool) {
	if a.InnerModel().ActivityLog.Has(nodeID) {
		return true
	}

	return
}

// String returns a human-readable version of ActivityLog.
func (a *ActivityLog) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("ActivityLog(len=%d, elements=", a.Size()))
	a.InnerModel().ActivityLog.ForEach(func(nodeID identity.ID) (err error) {
		builder.WriteString(fmt.Sprintf("%s, ", nodeID.String()))
		return
	})
	builder.WriteString(")")
	return builder.String()
}

// Clone clones the ActivityLog.
func (a *ActivityLog) Clone() *ActivityLog {
	clone := NewActivityLog()
	clone.InnerModel().ActivityLog = a.InnerModel().ActivityLog.Clone()
	return clone
}

// ForEach iterates through the activity set and calls the callback for every element.
func (a *ActivityLog) ForEach(callback func(nodeID identity.ID) (err error)) (err error) {
	return a.InnerModel().ActivityLog.ForEach(callback)
}

// Size returns the size of the activity log.
func (a *ActivityLog) Size() int {
	return a.InnerModel().ActivityLog.Size()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// SnapshotEpochActivity is the data structure to store node activity for the snapshot.
type SnapshotEpochActivity map[Index]*SnapshotNodeActivity

// NewSnapshotEpochActivity creates a new SnapshotEpochActivity instance.
func NewSnapshotEpochActivity() SnapshotEpochActivity {
	return make(SnapshotEpochActivity)
}

// SnapshotNodeActivity is structure to store nodes activity for an epoch.
type SnapshotNodeActivity struct {
	model.Mutable[SnapshotNodeActivity, *SnapshotNodeActivity, nodeActivityModel] `serix:"0"`
}

// NewSnapshotNodeActivity creates a new SnapshotNodeActivity instance.
func NewSnapshotNodeActivity() *SnapshotNodeActivity {
	return model.NewMutable[SnapshotNodeActivity](&nodeActivityModel{NodesLog: make(map[identity.ID]uint64)})
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
