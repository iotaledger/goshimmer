package activitylog

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(nodesActivitySerializableMap{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32))
	if err != nil {
		panic(fmt.Errorf("error registering NodesActivityLog type settings: %w", err))
	}
}

// region NodesActivityLog //////////////////////////////////////////////////////////////////////////////////////////////////

type nodesActivitySerializableMap map[epoch.Index]*ActivityLog

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
	shrinkingmap.ShrinkingMap[epoch.Index, *ActivityLog] `serix:"0,lengthPrefixType=uint32"`
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
	return &NodesActivityLog{*shrinkingmap.New[epoch.Index, *ActivityLog]()}
}

func (al *NodesActivityLog) activityLogsMap() *nodesActivitySerializableMap {
	activityMap := make(nodesActivitySerializableMap)
	al.ForEach(func(ei epoch.Index, activity *ActivityLog) bool {
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
type SnapshotEpochActivity map[epoch.Index]*SnapshotNodeActivity

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
