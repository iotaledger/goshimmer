package slotnotarization

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

// SlotMutations is an in-memory data structure that enables the collection of mutations for uncommitted slots.
type SlotMutations struct {
	weights *sybilprotection.Weights

	AcceptedBlockRemoved *event.Event1[models.BlockID]

	// acceptedBlocksBySlot stores the accepted blocks per slot.
	acceptedBlocksBySlot *shrinkingmap.ShrinkingMap[slot.Index, *ads.Set[models.BlockID, *models.BlockID]]

	// acceptedTransactionsBySlot stores the accepted transactions per slot.
	acceptedTransactionsBySlot *shrinkingmap.ShrinkingMap[slot.Index, *ads.Set[utxo.TransactionID, *utxo.TransactionID]]

	// latestCommittedIndex stores the index of the latest committed slot.
	latestCommittedIndex slot.Index

	evictionMutex sync.RWMutex

	// lastCommittedSlotCumulativeWeight stores the cumulative weight of the last committed slot
	lastCommittedSlotCumulativeWeight uint64
}

// NewSlotMutations creates a new SlotMutations instance.
func NewSlotMutations(weights *sybilprotection.Weights, lastCommittedSlot slot.Index) (newMutationFactory *SlotMutations) {
	return &SlotMutations{
		AcceptedBlockRemoved:       event.New1[models.BlockID](),
		weights:                    weights,
		acceptedBlocksBySlot:       shrinkingmap.New[slot.Index, *ads.Set[models.BlockID, *models.BlockID]](),
		acceptedTransactionsBySlot: shrinkingmap.New[slot.Index, *ads.Set[utxo.TransactionID, *utxo.TransactionID]](),
		latestCommittedIndex:       lastCommittedSlot,
	}
}

// AddAcceptedBlock adds the given block to the set of accepted blocks.
func (m *SlotMutations) AddAcceptedBlock(block *models.Block) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: slot with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index(), true).Add(blockID)

	return
}

// RemoveAcceptedBlock removes the given block from the set of accepted blocks.
func (m *SlotMutations) RemoveAcceptedBlock(block *models.Block) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: slot with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index()).Delete(blockID)

	m.AcceptedBlockRemoved.Trigger(blockID)

	return
}

// AddAcceptedTransaction adds the given transaction to the set of accepted transactions.
func (m *SlotMutations) AddAcceptedTransaction(metadata *mempool.TransactionMetadata) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	if metadata.InclusionSlot() <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed slot %d", metadata.ID(), metadata.InclusionSlot(), metadata.InclusionSlot())
	}

	m.acceptedTransactions(metadata.InclusionSlot(), true).Add(metadata.ID())

	return
}

// RemoveAcceptedTransaction removes the given transaction from the set of accepted transactions.
func (m *SlotMutations) RemoveAcceptedTransaction(metadata *mempool.TransactionMetadata) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	if metadata.InclusionSlot() <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed slot %d", metadata.ID(), metadata.InclusionSlot(), metadata.InclusionSlot())
	}

	m.acceptedTransactions(metadata.InclusionSlot(), false).Delete(metadata.ID())

	return
}

// UpdateTransactionInclusion moves a transaction from a later slot to the given slot.
func (m *SlotMutations) UpdateTransactionInclusion(txID utxo.TransactionID, oldSlot, newSlot slot.Index) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	if oldSlot <= m.latestCommittedIndex || newSlot <= m.latestCommittedIndex {
		return errors.Errorf("inclusion time of transaction changed for already committed slot: previous Index %d, new Index %d", oldSlot, newSlot)
	}

	m.acceptedTransactions(oldSlot, false).Delete(txID)
	m.acceptedTransactions(newSlot, true).Add(txID)

	return
}

// Evict evicts the given slot and returns the corresponding mutation sets.
func (m *SlotMutations) Evict(index slot.Index) (acceptedBlocks *ads.Set[models.BlockID, *models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID, *utxo.TransactionID], err error) {
	m.evictionMutex.Lock()
	defer m.evictionMutex.Unlock()

	if index <= m.latestCommittedIndex {
		return nil, nil, errors.Errorf("cannot commit slot %d: already committed", index)
	}

	defer m.evictUntil(index)

	return m.acceptedBlocks(index), m.acceptedTransactions(index), nil
}

func (m *SlotMutations) Reset(index slot.Index) {
	m.evictionMutex.Lock()
	defer m.evictionMutex.Unlock()

	for i := m.latestCommittedIndex; i > index; i-- {
		m.acceptedBlocksBySlot.Delete(i)
		m.acceptedTransactionsBySlot.Delete(i)
	}

	m.latestCommittedIndex = index
}

// acceptedBlocks returns the set of accepted blocks for the given slot.
func (m *SlotMutations) acceptedBlocks(index slot.Index, createIfMissing ...bool) *ads.Set[models.BlockID, *models.BlockID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedBlocksBySlot.GetOrCreate(index, newSet[models.BlockID, *models.BlockID]))
	}

	return lo.Return1(m.acceptedBlocksBySlot.Get(index))
}

// acceptedTransactions returns the set of accepted transactions for the given slot.
func (m *SlotMutations) acceptedTransactions(index slot.Index, createIfMissing ...bool) *ads.Set[utxo.TransactionID, *utxo.TransactionID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedTransactionsBySlot.GetOrCreate(index, newSet[utxo.TransactionID]))
	}

	return lo.Return1(m.acceptedTransactionsBySlot.Get(index))
}

// evictUntil removes all data for slots that are older than the given slot.
func (m *SlotMutations) evictUntil(index slot.Index) {
	for i := m.latestCommittedIndex + 1; i <= index; i++ {
		m.acceptedBlocksBySlot.Delete(i)
		m.acceptedTransactionsBySlot.Delete(i)
	}

	m.latestCommittedIndex = index
}

// newSet is a generic constructor for a new ads.Set.
func newSet[K any, KPtr constraints.MarshalablePtr[K]]() *ads.Set[K, KPtr] {
	return ads.NewSet[K, KPtr](mapdb.NewMapDB())
}
