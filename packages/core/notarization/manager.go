package notarization

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/mana"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the notarization manager.
type Manager struct {
	clock                       *clock.Clock
	tangle                      *tangle.Tangle
	ledger                      *ledger.Ledger
	consensus                   *consensus.Consensus
	epochCommitmentFactory      *EpochCommitmentFactory
	epochCommitmentFactoryMutex sync.RWMutex
	bootstrapMutex              sync.RWMutex
	options                     *ManagerOptions
	pendingConflictsCounters    *shrinkingmap.ShrinkingMap[epoch.Index, uint64]
	log                         *logger.Logger
	Events                      *Events
	bootstrapped                bool
}

// NewManager creates and returns a new notarization manager.
func NewManager(c *clock.Clock, t *tangle.Tangle, l *ledger.Ledger, consensusInstance *consensus.Consensus, epochCommitmentFactory *EpochCommitmentFactory, opts ...ManagerOption) (new *Manager) {
	options := &ManagerOptions{
		MinCommittableEpochAge: defaultMinEpochCommittableAge,
		Log:                    nil,
	}

	for _, option := range opts {
		option(options)
	}

	return &Manager{
		clock:                    c,
		tangle:                   t,
		ledger:                   l,
		consensus:                consensusInstance,
		epochCommitmentFactory:   epochCommitmentFactory,
		pendingConflictsCounters: shrinkingmap.New[epoch.Index, uint64](),
		log:                      options.Log,
		options:                  options,
		Events:                   NewEvents(),
	}
}

// StartSnapshot locks the commitment factory and returns the latest ecRecord and last confirmed epoch index.
func (m *Manager) StartSnapshot() (fullEpochIndex epoch.Index, ecRecord *chainmanager.Commitment, err error) {
	m.epochCommitmentFactoryMutex.RLock()

	latestConfirmedEpoch, err := m.LatestConfirmedEpochIndex()
	if err != nil {
		return
	}
	ecRecord = m.epochCommitmentFactory.loadECRecord(latestConfirmedEpoch)
	if ecRecord == nil {
		err = errors.Errorf("could not get latest commitment")
		return
	}

	// The snapshottable ledgerstate always sits at latestConfirmedEpoch - snapshotDepth
	fullEpochIndex = latestConfirmedEpoch - epoch.Index(m.epochCommitmentFactory.snapshotDepth)
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
		m.epochCommitmentFactory.storage.ledgerstateStorage.Store(outputWithMetadata).Release()
		err := insertLeaf(m.epochCommitmentFactory.stateRootTree, outputWithMetadata.ID().Bytes(), outputWithMetadata.ID().Bytes())
		if err != nil {
			m.log.Error(err)
		}
		err = m.epochCommitmentFactory.updateManaLeaf(outputWithMetadata, true)
		if err != nil {
			m.log.Error(err)
		}
	}
}

// LoadEpochDiff loads an epoch diff.
func (m *Manager) LoadEpochDiff(epochDiff *ledger.EpochDiff) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	for _, spentOutputWithMetadata := range epochDiff.Spent() {
		spentOutputIDBytes := spentOutputWithMetadata.ID().Bytes()
		if has := m.epochCommitmentFactory.storage.ledgerstateStorage.DeleteIfPresent(spentOutputIDBytes); !has {
			panic("epoch diff spends an output not contained in the ledger state")
		}
		if has, _ := m.epochCommitmentFactory.stateRootTree.Has(spentOutputIDBytes); !has {
			panic("epoch diff spends an output not contained in the state tree")
		}
		_, err := m.epochCommitmentFactory.stateRootTree.Delete(spentOutputIDBytes)
		if err != nil {
			panic("could not delete leaf from state root tree")
		}
	}
	for _, createdOutputWithMetadata := range epochDiff.Created() {
		createdOutputIDBytes := createdOutputWithMetadata.ID().Bytes()
		m.epochCommitmentFactory.storage.ledgerstateStorage.Store(createdOutputWithMetadata).Release()
		_, err := m.epochCommitmentFactory.stateRootTree.Update(createdOutputIDBytes, createdOutputIDBytes)
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
	if err := m.epochCommitmentFactory.storage.setLatestCommittableEpochIndex(header.DiffEpochIndex); err != nil {
		panic("could not set last committed epoch index")
	}

	// We assume as our earliest forking point the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.setLastConfirmedEpochIndex(header.DiffEpochIndex); err != nil {
		panic("could not set last confirmed epoch index")
	}

	// We set it to the next epoch after snapshotted one. It will be updated upon first confirmed block will arrive.
	if err := m.epochCommitmentFactory.storage.setAcceptanceEpochIndex(header.DiffEpochIndex + 1); err != nil {
		panic("could not set current epoch index")
	}

	commitmentToStore := chainmanager.NewCommitment(header.LatestECRecord.ID())
	commitmentToStore.PublishCommitment(header.LatestECRecord)

	m.epochCommitmentFactory.storage.ecRecordStorage.Store(commitmentToStore).Release()
}

// LoadActivityLogs loads activity logs from the snapshot and updates the activity tree.
func (m *Manager) LoadActivityLogs(epochActivity activitylog.SnapshotEpochActivity) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	for ei, nodeActivity := range epochActivity {
		for nodeID, acceptedCount := range nodeActivity.NodesLog() {
			err := m.epochCommitmentFactory.insertActivityLeaf(ei, nodeID, acceptedCount)
			if err != nil {
				m.log.Error(err)
			}
		}
	}
}

// SnapshotEpochDiffs returns the EpochDiffs when a snapshot is created.
func (m *Manager) SnapshotEpochDiffs(fullEpochIndex, latestCommitableEpoch epoch.Index, prodChan chan *ledger.EpochDiff, stopChan chan struct{}) {
	go func() {
		for ei := fullEpochIndex; ei <= latestCommitableEpoch; ei++ {
			spent, created := m.epochCommitmentFactory.loadDiffUTXOs(ei)
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
		m.epochCommitmentFactory.loadLedgerState(func(o *ledger.OutputWithMetadata) {
			prodChan <- o
		})
		close(stopChan)
	}()

	return
}

// GetLatestEC returns the latest commitment that a new block should commit to.
func (m *Manager) GetLatestEC() (ecRecord *chainmanager.Commitment, err error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	// TODO: remove as it's a quick fix
	//latestCommittableEpoch, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	ecRecord = m.epochCommitmentFactory.loadECRecord(0)
	if ecRecord == nil {
		err = errors.Errorf("could not get latest commitment")
	}
	return
}

// LatestCommitableEpochIndex returns the latest commitable epoch index.
func (m *Manager) LatestCommitableEpochIndex() (ecRecord epoch.Index, err error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	return m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
}

// LatestConfirmedEpochIndex returns the latest epoch index that has been confirmed.
func (m *Manager) LatestConfirmedEpochIndex() (epoch.Index, error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	return m.epochCommitmentFactory.storage.lastConfirmedEpochIndex()
}

// OnBlockAccepted is the handler for block confirmed event.
func (m *Manager) OnBlockAccepted(block *acceptance.Block) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(block.IssuingTime())

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("block %s accepted with issuing time %s in already committed epoch %d", block.ID(), block.IssuingTime(), ei)
		return
	}
	err := m.epochCommitmentFactory.insertTangleLeaf(ei, block.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
		return
	}
	m.updateEpochsBootstrapped(ei)

	m.Events.TangleTreeInserted.Trigger(&TangleTreeUpdatedEvent{EI: ei, BlockID: block.ID()})
}

// OnBlockAttached is a handler fo Block stored event that updates the activity log and triggers warpsyncing.
// TODO: notarization should not track activity of attached blocks, only of accepted blocks.
// TODO: the local perception of activity should be based on attachments instead.
// TODO: at the end of an epoch the local perception of the node will collpse to the same one of the notarization's, due
// TODO: to the orphanage. So attachements - orphanage = acceptance.
func (m *Manager) OnBlockAttached(block *blockdag.Block) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(block.IssuingTime())

	nodeID := identity.NewID(block.IssuerPublicKey())
	err := m.epochCommitmentFactory.insertActivityLeaf(ei, nodeID)
	if err != nil && m.log != nil {
		m.log.Error(err)
		return
	}
	m.Events.ActivityTreeInserted.Trigger(&ActivityTreeUpdatedEvent{EI: ei, NodeID: nodeID})

	blockEI := block.Commitment().Index()
	latestCommittableEI := lo.PanicOnErr(m.epochCommitmentFactory.storage.latestCommittableEpochIndex())
	epochDeltaSeconds := time.Duration(int64(blockEI-latestCommittableEI)*epoch.Duration) * time.Second

	// If we are too far behind, we will warpsync
	if epochDeltaSeconds > m.options.BootstrapWindow {
		m.Events.SyncRange.Trigger(&SyncRangeEvent{
			StartEI:   latestCommittableEI,
			EndEI:     blockEI,
			StartEC:   m.epochCommitmentFactory.loadECRecord(latestCommittableEI).ID(),
			EndPrevEC: block.Commitment().PrevID(),
		})
	}
}

// OnBlockOrphaned is the handler for block orphaned event.
func (m *Manager) OnBlockOrphaned(block *blockdag.Block) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(block.IssuingTime())
	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("block %s orphaned with issuing time %s in already committed epoch %d", block.ID(), block.IssuingTime(), ei)
		return
	}
	err := m.epochCommitmentFactory.removeTangleLeaf(ei, block.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}

	m.Events.TangleTreeRemoved.Trigger(&TangleTreeUpdatedEvent{EI: ei, BlockID: block.ID()})

	transaction, isTransaction := block.Payload().(utxo.Transaction)
	nodeID := identity.NewID(block.IssuerPublicKey())

	updatedCount := uint64(1)
	// if block has been accepted, counter was increased two times, on booking and on acceptance
	if m.consensus.Gadget.IsBlockAccepted(block.ID()) {
		updatedCount++
	}

	// TODO: we assume that the validator set of the engine for this epoch has been adjusted already, by taking into
	// account the orphaned block: i.e. removed the validator if it was the only issuer's block within the epoch.
	// Currently, the validator.Set is not epoch-scoped, but it should be: we want to make sure there is no activity
	// left from this nodeID in the epoch the block was orphaned from.
	//
	// OLD CODE:
	// noActivityLeft := m.tangle.WeightProvider.Remove(ei, nodeID, updatedCount)
	// if noActivityLeft {
	// 	err = m.epochCommitmentFactory.removeActivityLeaf(ei, nodeID)
	// 	if err != nil && m.log != nil {
	// 		m.log.Error(err)
	// 		return
	// 	}
	// 	m.Events.ActivityTreeRemoved.Trigger(&ActivityTreeUpdatedEvent{Index: ei, IssuerID: nodeID})
	// }

	if _, exists := m.tangle.ValidatorSet.Get(nodeID); !exists {
		err = m.epochCommitmentFactory.removeActivityLeaf(ei, nodeID)
		if err != nil && m.log != nil {
			m.log.Error(err)
			return
		}
		m.Events.ActivityTreeRemoved.Trigger(&ActivityTreeUpdatedEvent{EI: ei, NodeID: nodeID})
	}

	if isTransaction {
		spent, created := m.resolveOutputs(transaction)
		m.epochCommitmentFactory.deleteDiffUTXOs(ei, created, spent)
		m.Events.UTXOTreeRemoved.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})
	}
}

// OnTransactionAccepted is the handler for transaction accepted event.
func (m *Manager) OnTransactionAccepted(event *ledger.TransactionAcceptedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	txID := event.TransactionID

	var txInclusionTime time.Time
	m.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMeta *ledger.TransactionMetadata) {
		txInclusionTime = txMeta.InclusionTime()
	})
	txEpoch := epoch.IndexFromTime(txInclusionTime)

	if m.isEpochAlreadyCommitted(txEpoch) {
		m.log.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", event.TransactionID, txInclusionTime, txEpoch)
		return
	}

	var spent, created []*ledger.OutputWithMetadata
	m.ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		spent, created = m.resolveOutputs(tx)
	})
	if err := m.includeTransactionInEpoch(txID, txEpoch, spent, created); err != nil {
		m.log.Error(err)
	}
}

// OnTransactionInclusionUpdated is the handler for transaction inclusion updated event.
func (m *Manager) OnTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	oldEpoch := epoch.IndexFromTime(event.PreviousInclusionTime)
	newEpoch := epoch.IndexFromTime(event.InclusionTime)

	if oldEpoch == 0 || oldEpoch == newEpoch {
		return
	}

	if m.isEpochAlreadyCommitted(oldEpoch) || m.isEpochAlreadyCommitted(newEpoch) {
		m.log.Errorf("inclusion time of transaction changed for already committed epoch: previous Index %d, new Index %d", oldEpoch, newEpoch)
		return
	}

	txID := event.TransactionID

	has, err := m.isTransactionInEpoch(event.TransactionID, oldEpoch)
	if err != nil {
		m.log.Error(err)
		return
	}
	if !has {
		return
	}

	var spent, created []*ledger.OutputWithMetadata
	m.ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		spent, created = m.resolveOutputs(tx)
	})

	if err := m.removeTransactionFromEpoch(txID, oldEpoch, spent, created); err != nil {
		m.log.Error(err)
	}

	if err := m.includeTransactionInEpoch(txID, newEpoch, spent, created); err != nil {
		m.log.Error(err)
	}
}

// OnConflictAccepted is the handler for conflict confirmed event.
func (m *Manager) OnConflictAccepted(conflictID utxo.TransactionID) {
	epochCommittableEvents, manaVectorUpdateEvents := m.onConflictAccepted(conflictID)
	m.triggerEpochEvents(epochCommittableEvents, manaVectorUpdateEvents)
}

// OnConflictConfirmed is the handler for conflict confirmed event.
func (m *Manager) onConflictAccepted(conflictID utxo.TransactionID) ([]*EpochCommittableEvent, []*mana.ManaVectorUpdateEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("conflict confirmed in already committed epoch %d", ei)
		return nil, nil
	}
	return m.decreasePendingConflictCounter(ei)
}

// OnConflictCreated is the handler for conflict created event.
func (m *Manager) OnConflictCreated(conflictID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("conflict created in already committed epoch %d", ei)
		return
	}
	m.increasePendingConflictCounter(ei)
}

// OnConflictRejected is the handler for conflict created event.
func (m *Manager) OnConflictRejected(conflictID utxo.TransactionID) {
	epochCommittableEvents, manaVectorUpdateEvents := m.onConflictRejected(conflictID)
	m.triggerEpochEvents(epochCommittableEvents, manaVectorUpdateEvents)
}

// OnConflictRejected is the handler for conflict created event.
func (m *Manager) onConflictRejected(conflictID utxo.TransactionID) ([]*EpochCommittableEvent, []*mana.ManaVectorUpdateEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("conflict rejected in already committed epoch %d", ei)
		return nil, nil
	}
	return m.decreasePendingConflictCounter(ei)
}

// OnAcceptanceTimeUpdated is the handler for time updated event and triggers the events.
func (m *Manager) OnAcceptanceTimeUpdated(newTime time.Time) {
	epochCommittableEvents, manaVectorUpdateEvents := m.onAcceptanceTimeUpdated(newTime)
	m.triggerEpochEvents(epochCommittableEvents, manaVectorUpdateEvents)
}

// OnAcceptanceTimeUpdated is the handler for time updated event and returns events to be triggered.
func (m *Manager) onAcceptanceTimeUpdated(newTime time.Time) ([]*EpochCommittableEvent, []*mana.ManaVectorUpdateEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(newTime)
	currentEpochIndex, err := m.epochCommitmentFactory.storage.acceptanceEpochIndex()
	if err != nil {
		m.log.Error(errors.Wrap(err, "could not get current epoch index"))
		return nil, nil
	}
	// moved to the next epoch
	if ei > currentEpochIndex {
		err = m.epochCommitmentFactory.storage.setAcceptanceEpochIndex(ei)
		if err != nil {
			m.log.Error(errors.Wrap(err, "could not set current epoch index"))
			return nil, nil
		}
		return m.moveLatestCommittableEpoch(ei)
	}
	return nil, nil
}

// PendingConflictsCountAll returns the current value of pendingConflictsCount per epoch.
func (m *Manager) PendingConflictsCountAll() (pendingConflicts map[epoch.Index]uint64) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	pendingConflicts = make(map[epoch.Index]uint64, m.pendingConflictsCounters.Size())
	m.pendingConflictsCounters.ForEach(func(k epoch.Index, v uint64) bool {
		pendingConflicts[k] = v
		return true
	})
	return pendingConflicts
}

// Bootstrapped returns the current value of pendingConflictsCount per epoch.
func (m *Manager) Bootstrapped() bool {
	m.bootstrapMutex.RLock()
	defer m.bootstrapMutex.RUnlock()

	return m.bootstrapped
}

// Shutdown shuts down the manager's permanent storagee.
func (m *Manager) Shutdown() {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	m.epochCommitmentFactory.storage.shutdown()
}

func (m *Manager) decreasePendingConflictCounter(ei epoch.Index) ([]*EpochCommittableEvent, []*mana.ManaVectorUpdateEvent) {
	count, _ := m.pendingConflictsCounters.Get(ei)
	count--
	m.pendingConflictsCounters.Set(ei, count)
	if count == 0 {
		return m.moveLatestCommittableEpoch(ei)
	}
	return nil, nil
}

func (m *Manager) increasePendingConflictCounter(ei epoch.Index) {
	count, _ := m.pendingConflictsCounters.Get(ei)
	count++
	m.pendingConflictsCounters.Set(ei, count)
}

func (m *Manager) includeTransactionInEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.insertStateMutationLeaf(ei, txID); err != nil {
		return err
	}
	// TODO: in case of a reorg, a transaction spending the same output of another TX will cause a duplicate element
	// in cache in the objectstorage if we don't hook to the reorged transaction "orphanage".
	m.epochCommitmentFactory.storeDiffUTXOs(ei, spent, created)

	m.Events.StateMutationTreeInserted.Trigger(&StateMutationTreeUpdatedEvent{TransactionID: txID})
	m.Events.UTXOTreeInserted.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})

	return nil
}

func (m *Manager) removeTransactionFromEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.removeStateMutationLeaf(ei, txID); err != nil {
		return err
	}
	m.epochCommitmentFactory.deleteDiffUTXOs(ei, spent, created)

	m.Events.StateMutationTreeRemoved.Trigger(&StateMutationTreeUpdatedEvent{TransactionID: txID})
	m.Events.UTXOTreeRemoved.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})

	return nil
}

func (m *Manager) isTransactionInEpoch(txID utxo.TransactionID, ei epoch.Index) (has bool, err error) {
	return m.epochCommitmentFactory.hasStateMutationLeaf(ei, txID)
}

// isCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) isCommittable(ei epoch.Index) bool {
	return m.isOldEnough(ei) && m.allPastConflictsAreResolved(ei)
}

func (m *Manager) allPastConflictsAreResolved(ei epoch.Index) (conflictsResolved bool) {
	lastEI, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		return false
	}
	// epoch is not committable if there are any not resolved conflicts in this and past epochs
	for index := lastEI; index <= ei; index++ {
		if count, _ := m.pendingConflictsCounters.Get(index); count != 0 {
			return false
		}
	}
	return true
}

func (m *Manager) isOldEnough(ei epoch.Index, issuingTime ...time.Time) (oldEnough bool) {
	t := ei.EndTime()
	currentATT := m.clock.AcceptedTime()
	if len(issuingTime) > 0 && issuingTime[0].After(currentATT) {
		currentATT = issuingTime[0]
	}

	diff := currentATT.Sub(t)
	if diff < m.options.MinCommittableEpochAge {
		return false
	}
	return true
}

func (m *Manager) getConflictEI(conflictID utxo.TransactionID) epoch.Index {
	earliestAttachment := m.tangle.GetEarliestAttachment(conflictID)
	return epoch.IndexFromTime(earliestAttachment.IssuingTime())
}

func (m *Manager) isEpochAlreadyCommitted(ei epoch.Index) bool {
	latestCommittable, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		m.log.Errorf("could not get the latest committed epoch: %v", err)
		return false
	}
	return ei <= latestCommittable
}

func (m *Manager) resolveOutputs(tx utxo.Transaction) (spentOutputsWithMetadata, createdOutputsWithMetadata []*ledger.OutputWithMetadata) {
	spentOutputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	createdOutputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	var spentOutputIDs utxo.OutputIDs
	var createdOutputs []utxo.Output

	spentOutputIDs = m.ledger.Utils.ResolveInputs(tx.Inputs())
	createdOutputs = tx.(*devnetvm.Transaction).Essence().Outputs().UTXOOutputs()

	for it := spentOutputIDs.Iterator(); it.HasNext(); {
		spentOutputID := it.Next()
		m.ledger.Storage.CachedOutput(spentOutputID).Consume(func(spentOutput utxo.Output) {
			m.ledger.Storage.CachedOutputMetadata(spentOutputID).Consume(func(spentOutputMetadata *ledger.OutputMetadata) {
				spentOutputsWithMetadata = append(spentOutputsWithMetadata, ledger.NewOutputWithMetadata(spentOutputID, spentOutput, spentOutputMetadata.CreationTime(), spentOutputMetadata.ConsensusManaPledgeID(), spentOutputMetadata.AccessManaPledgeID()))
			})
		})
	}

	for _, createdOutput := range createdOutputs {
		createdOutputID := createdOutput.ID()
		m.ledger.Storage.CachedOutputMetadata(createdOutputID).Consume(func(createdOutputMetadata *ledger.OutputMetadata) {
			createdOutputsWithMetadata = append(createdOutputsWithMetadata, ledger.NewOutputWithMetadata(createdOutputID, createdOutput, createdOutputMetadata.CreationTime(), createdOutputMetadata.ConsensusManaPledgeID(), createdOutputMetadata.AccessManaPledgeID()))
		})
	}

	return
}

func (m *Manager) manaVectorUpdate(ei epoch.Index) (event *mana.ManaVectorUpdateEvent) {
	manaEpoch := ei - epoch.Index(m.options.ManaEpochDelay)
	spent := []*ledger.OutputWithMetadata{}
	created := []*ledger.OutputWithMetadata{}

	if manaEpoch > 0 {
		spent, created = m.epochCommitmentFactory.loadDiffUTXOs(manaEpoch)
	}

	return &mana.ManaVectorUpdateEvent{
		EI:      ei,
		Spent:   spent,
		Created: created,
	}
}

func (m *Manager) moveLatestCommittableEpoch(currentEpoch epoch.Index) ([]*EpochCommittableEvent, []*mana.ManaVectorUpdateEvent) {
	latestCommittable, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		m.log.Errorf("could not obtain last committed epoch index: %v", err)
		return nil, nil
	}

	epochCommittableEvents := make([]*EpochCommittableEvent, 0)
	manaVectorUpdateEvents := make([]*mana.ManaVectorUpdateEvent, 0)
	for ei := latestCommittable + 1; ei <= currentEpoch; ei++ {
		if !m.isCommittable(ei) {
			break
		}

		// reads the roots and store the ec
		// rolls the state trees
		ecRecord, ecRecordErr := m.epochCommitmentFactory.ecRecord(ei)
		if ecRecordErr != nil {
			m.log.Errorf("could not update commitments for epoch %d: %v", ei, ecRecordErr)
			return nil, nil
		}

		if err = m.epochCommitmentFactory.storage.setLatestCommittableEpochIndex(ei); err != nil {
			m.log.Errorf("could not set last committed epoch: %v", err)
			return nil, nil
		}

		epochCommittableEvents = append(epochCommittableEvents, &EpochCommittableEvent{
			EI:       ei,
			ECRecord: ecRecord,
		})
		if manaVectorUpdateEvent := m.manaVectorUpdate(ei); manaVectorUpdateEvent != nil {
			manaVectorUpdateEvents = append(manaVectorUpdateEvents, manaVectorUpdateEvent)
		}

		// We do not need to track pending conflicts for a committed epoch anymore.
		m.pendingConflictsCounters.Delete(ei)
	}
	return epochCommittableEvents, manaVectorUpdateEvents
}

func (m *Manager) triggerEpochEvents(epochCommittableEvents []*EpochCommittableEvent, manaVectorUpdateEvents []*mana.ManaVectorUpdateEvent) {
	for _, epochCommittableEvent := range epochCommittableEvents {
		m.Events.EpochCommittable.Trigger(epochCommittableEvent)
	}
	for _, manaVectorUpdateEvent := range manaVectorUpdateEvents {
		m.Events.ManaVectorUpdate.Trigger(manaVectorUpdateEvent)
	}
}

func (m *Manager) updateEpochsBootstrapped(ei epoch.Index) {
	if !m.Bootstrapped() &&
		(ei > epoch.IndexFromTime(m.clock.RelativeAcceptedTime().Add(-m.options.BootstrapWindow)) ||
			m.options.BootstrapWindow == 0) {
		m.bootstrapMutex.Lock()
		m.bootstrapped = true
		m.bootstrapMutex.Unlock()
		m.Events.Bootstrapped.Trigger(&BootstrappedEvent{})
	}
}

// SnapshotEpochActivity snapshots accepted block counts from activity tree and updates provided SnapshotEpochActivity.
func (m *Manager) SnapshotEpochActivity(epochDiffIndex epoch.Index) (epochActivity activitylog.SnapshotEpochActivity, err error) {
	// TODO: obtain activity for epoch from the sybilprotection component embedded in the Engine
	// epochActivity = m.tangle.WeightProvider.SnapshotEpochActivity(epochDiffIndex)
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// ManagerOption represents the return type of the optional config parameters of the notarization manager.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container of all the config parameters of the notarization manager.
type ManagerOptions struct {
	MinCommittableEpochAge time.Duration
	BootstrapWindow        time.Duration
	Log                    *logger.Logger
	ManaEpochDelay         uint
}

// ManaEpochDelay specifies how many epochs the consensus mana booking is delayed with respect to the latest committable
// epoch.
func ManaEpochDelay(manaEpochDelay uint) ManagerOption {
	return func(options *ManagerOptions) {
		options.ManaEpochDelay = manaEpochDelay
	}
}

// MinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func MinCommittableEpochAge(d time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.MinCommittableEpochAge = d
	}
}

// BootstrapWindow specifies when the notarization manager is considered to be bootstrapped.
func BootstrapWindow(d time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.BootstrapWindow = d
	}
}

// Log provides the logger.
func Log(log *logger.Logger) ManagerOption {
	return func(options *ManagerOptions) {
		options.Log = log
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
