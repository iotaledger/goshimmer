package mana

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
)

// region Snapshot /////////////////////////////////////////////////////////////////////////////////////////////////////

// Snapshot defines a snapshot of the ledger state.
type Snapshot struct {
	ByNodeID map[identity.ID]*SnapshotNode
}

/*

	snapshot := deps.Tangle.Ledger.SnapshotUTXO()

	aMana, err := snapshotAccessMana()
	if err != nil {
		return err
	}
	snapshot.AccessManaByNode = aMana
*/

func (s *Snapshot) NodeSnapshot(nodeID identity.ID) (nodeSnapshot *SnapshotNode) {
	nodeSnapshot, exists := s.ByNodeID[nodeID]
	if exists {
		return nodeSnapshot
	}

	nodeSnapshot = &SnapshotNode{
		AccessMana:       AccessManaSnapshot{},
		SortedTxSnapshot: SortedTxSnapshot{},
	}
	s.ByNodeID[nodeID] = nodeSnapshot

	return nodeSnapshot
}

func (s *Snapshot) MaxAccessManaUpdateTime() (maxTime time.Time) {
	for _, node := range s.ByNodeID {
		if nodeMaxTime := node.AccessManaUpdateTime(); nodeMaxTime.After(maxTime) {
			maxTime = node.AccessManaUpdateTime()
		}
	}

	return maxTime
}

func (s *Snapshot) ResetTime() {
	addTime := time.Since(s.MaxAccessManaUpdateTime())
	for _, node := range s.ByNodeID {
		node.AdjustAccessManaUpdateTime(addTime)
	}
}

func (s *Snapshot) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	nodeCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("could not read node count: %w", err)
	}

	s.ByNodeID = make(map[identity.ID]*SnapshotNode, nodeCount)
	for i := uint64(0); i < nodeCount; i++ {
		var nodeId identity.ID
		nodeId, err = identity.IDFromMarshalUtil(marshalUtil)
		if err != nil {
			return errors.Errorf("could not read node id: %w", err)
		}

		var snapshotNode SnapshotNode
		if err = snapshotNode.FromMarshalUtil(marshalUtil); err != nil {
			return errors.Errorf("could not read snapshot of node: %w", err)
		}

		s.ByNodeID[nodeId] = &snapshotNode
	}

	return nil
}

func (s *Snapshot) Bytes() (serialized []byte) {
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AccessManaRecord /////////////////////////////////////////////////////////////////////////////////////////////

// AccessManaRecord defines the info for the aMana snapshot.
type AccessManaRecord struct {
	Value     float64
	Timestamp time.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
