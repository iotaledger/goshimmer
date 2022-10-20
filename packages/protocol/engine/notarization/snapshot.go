package notarization

/*

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// StartSnapshot locks the commitment factory and returns the latest ecRecord and last confirmed epoch index.
func (m *Manager) StartSnapshot() (fullEpochIndex epoch.Index, ecRecord *chainmanager.Commitment, err error) {
	m.epochCommitmentFactoryMutex.RLock()

	latestConfirmedEpoch, err := m.LatestConfirmedEpochIndex()
	if err != nil {
		return
	}
	ecRecord = m.commitmentFactory.loadECRecord(latestConfirmedEpoch)
	if ecRecord == nil {
		err = errors.Errorf("could not get latest commitment")
		return
	}

	// The snapshottable ledgerstate always sits at latestConfirmedEpoch - snapshotDepth
	fullEpochIndex = latestConfirmedEpoch - epoch.Index(m.commitmentFactory.snapshotDepth)
	if fullEpochIndex < 0 {
		fullEpochIndex = 0
	}

	return
}

// EndSnapshot unlocks the commitment factory when the snapshotting completes.
func (m *Manager) EndSnapshot() {
	m.epochCommitmentFactoryMutex.RUnlock()
}

// LoadOutputsWithMetadata initiates the state and mana trees from a given snapshot.
func (m *Manager) LoadOutputsWithMetadata(outputsWithMetadatas []*ledger.OutputWithMetadata) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	for _, outputWithMetadata := range outputsWithMetadatas {
		m.commitmentFactory.storage.ledgerstateStorage.Store(outputWithMetadata).Release()

		if _, err := m.commitmentFactory.stateRootTree.Update(outputWithMetadata.ID().Bytes(), outputWithMetadata.ID().Bytes()); err != nil {
			m.Events.Error.Trigger(err)
		}

		if err := m.commitmentFactory.updateManaLeaf(outputWithMetadata, true); err != nil {
			m.Events.Error.Trigger(err)
		}
	}
}

// LoadEpochDiff loads an epoch diff.
func (m *Manager) LoadEpochDiff(epochDiff *ledger.EpochDiff) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	for _, spentOutputWithMetadata := range epochDiff.Spent() {
		spentOutputIDBytes := spentOutputWithMetadata.ID().Bytes()
		if has := m.commitmentFactory.storage.ledgerstateStorage.DeleteIfPresent(spentOutputIDBytes); !has {
			panic("epoch diff spends an output not contained in the ledger state")
		}
		if has, _ := m.commitmentFactory.stateRootTree.Has(spentOutputIDBytes); !has {
			panic("epoch diff spends an output not contained in the state tree")
		}
		_, err := m.commitmentFactory.stateRootTree.Delete(spentOutputIDBytes)
		if err != nil {
			panic("could not delete leaf from state root tree")
		}
	}
	for _, createdOutputWithMetadata := range epochDiff.Created() {
		createdOutputIDBytes := createdOutputWithMetadata.ID().Bytes()
		m.commitmentFactory.storage.ledgerstateStorage.Store(createdOutputWithMetadata).Release()
		_, err := m.commitmentFactory.stateRootTree.Update(createdOutputIDBytes, createdOutputIDBytes)
		if err != nil {
			panic("could not update leaf of state root tree")
		}
	}

	return
}

// LoadECandEIs initiates the ECRecord, latest committable Index, last confirmed Index and acceptance Index from a given snapshot.
func (m *Manager) LoadECandEIs(header *ledger.SnapshotHeader) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	// The last committed epoch index corresponds to the last epoch diff stored in the snapshot.
	if err := m.commitmentFactory.storage.setLatestCommittableEpochIndex(header.DiffEpochIndex); err != nil {
		panic("could not set last committed epoch index")
	}

	// We assume as our earliest forking point the last epoch diff stored in the snapshot.
	if err := m.commitmentFactory.storage.setLastConfirmedEpochIndex(header.DiffEpochIndex); err != nil {
		panic("could not set last confirmed epoch index")
	}

	// We set it to the next epoch after snapshotted one. It will be updated upon first confirmed block will arrive.
	if err := m.commitmentFactory.storage.setAcceptanceEpochIndex(header.DiffEpochIndex + 1); err != nil {
		panic("could not set current epoch index")
	}

	commitmentToStore := chainmanager.NewCommitment(header.LatestECRecord.ID())
	commitmentToStore.PublishCommitment(header.LatestECRecord)

	m.commitmentFactory.storage.ecRecordStorage.Store(commitmentToStore).Release()
}

// LoadActivityLogs loads activity logs from the snapshot and updates the activity tree.
func (m *Manager) LoadActivityLogs(epochActivity activitylog.SnapshotEpochActivity) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	for ei, nodeActivity := range epochActivity {
		for nodeID, acceptedCount := range nodeActivity.NodesLog() {
			err := m.commitmentFactory.insertActivityLeaf(ei, nodeID, acceptedCount)
			if err != nil {
				m.Events.Error.Trigger(err)
			}
		}
	}
}

// SnapshotEpochDiffs returns the EpochDiffs when a snapshot is created.
func (m *Manager) SnapshotEpochDiffs(fullEpochIndex, latestCommitableEpoch epoch.Index, prodChan chan *ledger.EpochDiff, stopChan chan struct{}) {
	go func() {
		for ei := fullEpochIndex; ei <= latestCommitableEpoch; ei++ {
			spent, created := m.commitmentFactory.loadDiffUTXOs(ei)
			prodChan <- ledger.NewEpochDiff(spent, created)
		}

		close(stopChan)
	}()

	return
}

// SnapshotLedgerState returns the all confirmed OutputsWithMetadata when a snapshot is created.
func (m *Manager) SnapshotLedgerState(prodChan chan *ledger.OutputWithMetadata, stopChan chan struct{}) {
	// No need to lock because this is called in the context of a StartSnapshot.
	go func() {
		m.commitmentFactory.loadLedgerState(func(o *ledger.OutputWithMetadata) {
			prodChan <- o
		})
		close(stopChan)
	}()

	return
}

// SnapshotEpochActivity snapshots accepted block counts from activity tree and updates provided SnapshotEpochActivity.
func (m *Manager) SnapshotEpochActivity(epochDiffIndex epoch.Index) (epochActivity activitylog.SnapshotEpochActivity, err error) {
	// TODO: obtain activity for epoch from the sybilprotection component embedded in the Engine
	// epochActivity = m.tangle.WeightProvider.SnapshotEpochActivity(epochDiffIndex)
	return
}

*/
