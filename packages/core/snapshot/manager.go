package snapshot

import (
	"errors"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/kvstore"
)

const (
	prefixSolidEntryPoint byte = iota
)

type Manager struct {
	tangle          *tangleold.Tangle
	notarizationMgr *notarization.Manager

	baseStore kvstore.KVStore
	sepsLock  sync.RWMutex
}

func NewManager(store kvstore.KVStore, t *tangleold.Tangle, nmgr *notarization.Manager) (new *Manager) {
	new = &Manager{
		tangle:          t,
		notarizationMgr: nmgr,
	}

	new.baseStore = store

	new.tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(e *tangleold.BlockAcceptedEvent) {
		e.Block.ForEachParent(func(parent tangleold.Parent) {
			index := parent.ID.EpochIndex
			if index < e.Block.EI() {
				new.insertSolidEntryPoint(parent.ID)
			}
		})
	}))

	new.tangle.ConfirmationOracle.Events().BlockOrphaned.Attach(event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
		new.removeSolidEntryPoint(event.Block, event.Block.LatestConfirmedEpoch())
	}))

	return
}

func (m *Manager) CreateSnapshot(snapshotFileName string) (header *ledger.SnapshotHeader, err error) {
	ecRecord, lastConfirmedEpoch, err := m.tangle.Options.CommitmentFunc()
	if err != nil {
		return nil, err
	}

	// lock the entire ledger in notarization manager until the snapshot is created.
	m.notarizationMgr.WriteLockLedger()
	defer m.notarizationMgr.WriteUnlockLedger()

	headerPord := func() (header *ledger.SnapshotHeader, err error) {
		header = &ledger.SnapshotHeader{
			FullEpochIndex: lastConfirmedEpoch,
			DiffEpochIndex: ecRecord.EI(),
			LatestECRecord: ecRecord,
		}
		return header, nil
	}

	sepsProd := NewSolidEntryPointsProducer(lastConfirmedEpoch, ecRecord.EI(), m)
	outputWithMetadataProd := NewLedgerUTXOStatesProducer(lastConfirmedEpoch, m.notarizationMgr)
	epochDiffsProd := NewEpochDiffsProducer(lastConfirmedEpoch, ecRecord.EI(), m.notarizationMgr)

	header, err = CreateSnapshot(snapshotFileName, headerPord, sepsProd, outputWithMetadataProd, epochDiffsProd)

	return
}

func (m *Manager) LoadSolidEntryPoints(seps *SolidEntryPoints) {
	if seps == nil {
		return
	}

	for _, b := range seps.Seps {
		m.insertSolidEntryPoint(b)
	}
}

func (m *Manager) insertSolidEntryPoint(id tangleold.BlockID) error {
	m.sepsLock.Lock()
	defer m.sepsLock.Unlock()

	sepsStore, err := m.baseStore.WithRealm(append([]byte{database.PrefixSnapshot, prefixSolidEntryPoint}, id.EpochIndex.Bytes()...))
	if err != nil {
		panic(err)
	}

	if err := sepsStore.Set(id.Bytes(), id.Bytes()); err != nil {
		return errors.New("Fail to insert block to epoch store")
	}

	return nil
}

func (m *Manager) removeSolidEntryPoint(b *tangleold.Block, lastConfirmedEpoch epoch.Index) (err error) {
	m.sepsLock.Lock()
	defer m.sepsLock.Unlock()

	sepsStore, err := m.baseStore.WithRealm(append([]byte{database.PrefixSnapshot, prefixSolidEntryPoint}, b.EI().Bytes()...))
	if err != nil {
		panic(err)
	}

	idBytes, err := sepsStore.Get(b.ID().Bytes())
	if err != nil || idBytes == nil {
		return errors.New("solid entry point doesn't exist in storage or fail to fetch it")
	}

	var blkID tangleold.BlockID
	_, err = blkID.FromBytes(idBytes)
	if err != nil {
		return err
	}

	// cannot remove sep from confirmed epoch
	if blkID.EpochIndex < lastConfirmedEpoch {
		return
	}

	err = sepsStore.Delete(b.ID().Bytes())
	if err != nil {
		return err
	}

	return
}

// SolidEntryPointsOfEpoch dumps solid entry points within given epochs.
func (m *Manager) SolidEntryPointsOfEpoch(ei epoch.Index) (seps []tangleold.BlockID) {
	m.sepsLock.RLock()
	defer m.sepsLock.RUnlock()

	sepsPrefix := append([]byte{database.PrefixSnapshot, prefixSolidEntryPoint}, ei.Bytes()...)

	m.baseStore.IterateKeys(sepsPrefix, func(key kvstore.Key) bool {
		var blkID tangleold.BlockID
		_, err := blkID.FromBytes(key)
		if err != nil {
			return false
		}
		seps = append(seps, blkID)

		return true
	})

	return
}
