package booker

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

type Attachments struct {
	storage    *lockableMap[utxo.TransactionID, *lockableMap[epoch.Index, []*Block]]
	pruningMap *lockableMap[epoch.Index, utxo.TransactionIDs]

	sync.RWMutex
}

func (a *Attachments) Store(tx utxo.Transaction, block *Block) {
	a.RLock()
	defer a.RUnlock()

	a.storeAttachment(a.attachmentsStorage(tx.ID(), true), block)
	a.updatePruningMap(block.ID().EpochIndex, tx.ID())
}

func (a *Attachments) Attachments(txID utxo.TransactionID) (attachments []*Block) {
	a.RLock()
	defer a.RUnlock()

	return a.attachments(a.attachmentsStorage(txID, false))
}

func (a *Attachments) attachments(storage *lockableMap[epoch.Index, []*Block]) (attachments []*Block) {
	storage.RLock()
	defer storage.RUnlock()

	storage.ForEach(func(_ epoch.Index, blocks []*Block) bool {
		attachments = append(attachments, blocks...)
		return true
	})

	return
}

func (a *Attachments) storeAttachment(attachments *lockableMap[epoch.Index, []*Block], block *Block) {
	attachments.Lock()
	defer attachments.Unlock()

	attachmentsOfEpoch, exists := attachments.Get(block.ID().EpochIndex)
	if !exists {
		attachmentsOfEpoch = make([]*Block, 0)
	}
	attachments.Set(block.ID().EpochIndex, append(attachmentsOfEpoch, block))
}

func (a *Attachments) attachmentsStorage(txID utxo.TransactionID, createIfMissing bool) (storage *lockableMap[epoch.Index, []*Block]) {
	a.storage.Lock()
	defer a.storage.Unlock()

	storage, exists := a.storage.Get(txID)
	if exists {
		return
	}
	storage = newLockableMap[epoch.Index, []*Block]()

	if createIfMissing {
		a.storage.Set(txID, storage)
	}

	return
}

func (a *Attachments) updatePruningMap(epochIndex epoch.Index, txID utxo.TransactionID) {
	a.pruningMap.Lock()
	defer a.pruningMap.Unlock()

	a.txIDsPerEpoch(epochIndex).Add(txID)
}

func (a *Attachments) txIDsPerEpoch(epochIndex epoch.Index) (txIDs utxo.TransactionIDs) {
	txIDs, exists := a.pruningMap.Get(epochIndex)
	if exists {
		return txIDs
	}

	txIDs = utxo.NewTransactionIDs()
	a.pruningMap.Set(epochIndex, txIDs)

	return
}
