package notarization

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type mutationFactory struct {
	commitmentTrees         *shrinkingmap.ShrinkingMap[epoch.Index, *commitmentTrees]
	acceptedBlocksByIssuer  *shrinkingmap.ShrinkingMap[epoch.Index, map[identity.ID]*set.AdvancedSet[models.BlockID]]
	lastCommittedEpochIndex epoch.Index
}

func newMutationFactory(lastCommittedEpoch epoch.Index) (newMutationFactory *mutationFactory) {
	return &mutationFactory{
		commitmentTrees:         shrinkingmap.New[epoch.Index, *commitmentTrees](),
		acceptedBlocksByIssuer:  shrinkingmap.New[epoch.Index, map[identity.ID]*set.AdvancedSet[models.BlockID]](),
		lastCommittedEpochIndex: lastCommittedEpoch,
	}
}

func (m *mutationFactory) commit(ei epoch.Index) (tangleRoot, stateMutationRoot, activityRoot types.Identifier, err error) {
	commitmentTrees, err := m.getCommitmentTrees(ei)
	if err != nil {
		return tangleRoot, stateMutationRoot, activityRoot, errors.Errorf("could not get commitment while committing epoch %d: %w", ei, err)
	}

	m.lastCommittedEpochIndex = ei
	m.evict(ei)

	return commitmentTrees.TangleRoot(), commitmentTrees.StateMutationRoot(), commitmentTrees.ActivityRoot(), nil
}

func (m *mutationFactory) addAcceptedBlock(id identity.ID, blockID models.BlockID) {
	acceptedBlocks, exists := m.acceptedBlocksByIssuer.Get(blockID.Index())
	if !exists {
		acceptedBlocks = make(map[identity.ID]*set.AdvancedSet[models.BlockID])
		m.acceptedBlocksByIssuer.Set(blockID.Index(), acceptedBlocks)
	}

	acceptedBlocksByIssuerID, exists := acceptedBlocks[id]
	if !exists {
		acceptedBlocksByIssuerID = set.NewAdvancedSet[models.BlockID]()
		acceptedBlocks[id] = acceptedBlocksByIssuerID
	}

	if acceptedBlocksByIssuerID.Size() == 0 && acceptedBlocksByIssuerID.Add(blockID) {
		m.insertActivityLeaf(blockID.Index(), id)
	}
}

func (m *mutationFactory) removeAcceptedBlock(id identity.ID, blockID models.BlockID) {
	if acceptedBlocks, exists := m.acceptedBlocksByIssuer.Get(blockID.Index()); exists {
		if blocksByID, exists := acceptedBlocks[id]; exists {
			if blocksByID.Delete(blockID) && blocksByID.Size() == 0 {
				m.removeActivityLeaf(blockID.Index(), id)
				// TODO: TRIGGER ACTIVITY LEAF REMOVED
			}
		}
	}
}

func (m *mutationFactory) getCommitmentTrees(ei epoch.Index) (commitmentTrees *commitmentTrees, err error) {
	if ei <= m.lastCommittedEpochIndex {
		return nil, errors.Errorf("cannot get commitment trees for epoch %d, because it is already committed", ei)
	}
	commitmentTrees, ok := m.commitmentTrees.Get(ei)
	if !ok {
		commitmentTrees = newCommitmentTrees(ei)
		m.commitmentTrees.Set(ei, commitmentTrees)
	}
	return
}

// insertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (m *mutationFactory) insertStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state mutation leaf")
	}

	return lo.Return2(commitment.stateMutationTree.Update(txID.Bytes(), txID.Bytes()))
}

// removeStateMutationLeaf deletes the transaction ID to the state mutation sparse merkle tree.
func (m *mutationFactory) removeStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) error {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting state mutation leaf")
	}
	return lo.Return2(commitment.stateMutationTree.Delete(txID.Bytes()))
}

// hasStateMutationLeaf returns if the leaf is part of the state mutation sparse merkle tree.
func (m *mutationFactory) hasStateMutationLeaf(ei epoch.Index, txID utxo.TransactionID) (has bool, err error) {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return false, errors.Wrap(err, "could not get commitment while deleting state mutation leaf")
	}
	return commitment.stateMutationTree.Has(txID.Bytes())
}

// insertTangleLeaf inserts blk to the Tangle sparse merkle tree.
func (m *mutationFactory) insertTangleLeaf(ei epoch.Index, blkID models.BlockID) error {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting tangle leaf")
	}
	return lo.Return2(commitment.tangleTree.Update(lo.PanicOnErr(blkID.Bytes()), lo.PanicOnErr(blkID.Bytes())))
}

// removeTangleLeaf removes the block ID from the Tangle sparse merkle tree.
func (m *mutationFactory) removeTangleLeaf(ei epoch.Index, blkID models.BlockID) error {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting tangle leaf")
	}

	return lo.Return2(commitment.tangleTree.Delete(lo.PanicOnErr(blkID.Bytes())))
}

// insertActivityLeaf inserts nodeID to the Activity sparse merkle tree.
func (m *mutationFactory) insertActivityLeaf(ei epoch.Index, nodeID identity.ID) error {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting activity leaf")
	}

	return lo.Return2(commitment.activityTree.Update(nodeID.Bytes(), nodeID.Bytes()))
}

// removeActivityLeaf removes the nodeID from the Activity sparse merkle tree.
func (m *mutationFactory) removeActivityLeaf(ei epoch.Index, nodeID identity.ID) error {
	commitment, err := m.getCommitmentTrees(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting activity leaf")
	}

	return lo.Return2(commitment.activityTree.Delete(nodeID.Bytes()))
}

func (m *mutationFactory) evict(ei epoch.Index) {
	m.commitmentTrees.Delete(ei)
	m.acceptedBlocksByIssuer.Delete(ei)
}
