package snapshot

import (
	"errors"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/kvstore"
)

const (
	prefixSolidEntryPoint byte = iota
)

// Manager is the snapshot manager.
type Manager struct {
	sync.RWMutex

	notarizationMgr *notarization.Manager
	baseStore       kvstore.KVStore
}

// NewManager creates and returns a new snapshot manager.
func NewManager(store kvstore.KVStore, nmgr *notarization.Manager) (new *Manager) {
	new = &Manager{
		notarizationMgr: nmgr,
	}

	new.baseStore = store

	return
}

// CreateSnapshot creates a snapshot file from node with a given name.
func (m *Manager) CreateSnapshot(snapshotFileName string) (header *ledger.SnapshotHeader, err error) {
	lastConfirmedEpoch, err := m.notarizationMgr.LatestConfirmedEpochIndex()
	if err != nil {
		return nil, err
	}

	ecRecord, err := m.notarizationMgr.GetLatestEC()
	if err != nil {
		return nil, err
	}

	// lock the entire ledger in notarization manager until the snapshot is created.
	m.notarizationMgr.WriteLockLedger()
	defer m.notarizationMgr.WriteUnlockLedger()

	// lock the entire solid entry points storage until the snapshot is created.
	m.Lock()
	defer m.Unlock()

	headerProd := func() (header *ledger.SnapshotHeader, err error) {
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

	header, err = CreateSnapshot(snapshotFileName, headerProd, sepsProd, outputWithMetadataProd, epochDiffsProd)

	return
}

// LoadSolidEntryPoints add solid entry points to storage.
func (m *Manager) LoadSolidEntryPoints(seps *SolidEntryPoints) {
	if seps == nil {
		return
	}

	for _, b := range seps.Seps {
		m.InsertSolidEntryPoint(b)
	}
}

func (m *Manager) InsertSolidEntryPoint(id tangleold.BlockID) error {
	m.Lock()
	defer m.Unlock()

	prefix := byteutils.ConcatBytes([]byte{database.PrefixSnapshot, prefixSolidEntryPoint}, id.Bytes())
	sepsStore, err := m.baseStore.WithRealm(prefix)
	if err != nil {
		panic(err)
	}

	if err := sepsStore.Set(id.Bytes(), id.Bytes()); err != nil {
		return errors.New("Fail to insert block to epoch store")
	}

	return nil
}

func (m *Manager) RemoveSolidEntryPoint(b *tangleold.Block, lastConfirmedEpoch epoch.Index) (err error) {
	m.Lock()
	defer m.Unlock()

	prefix := byteutils.ConcatBytes([]byte{database.PrefixSnapshot, prefixSolidEntryPoint}, b.EI().Bytes())
	sepsStore, err := m.baseStore.WithRealm(prefix)
	if err != nil {
		panic(err)
	}

	idBytes, err := sepsStore.Get(b.ID().Bytes())
	if err != nil || idBytes == nil {
		return errors.New("solid entry point doesn't exist in storage or fail to fetch it")
	}

	var blkID tangleold.BlockID
	if _, err = blkID.FromBytes(idBytes); err != nil {
		return err
	}

	// cannot remove sep from confirmed epoch
	if blkID.EpochIndex <= lastConfirmedEpoch {
		return errors.New("try to remove a solid entry point of confirmed epoch")
	}

	if err = sepsStore.Delete(b.ID().Bytes()); err != nil {
		return err
	}

	return
}

func (m *Manager) SnapshotSolidEntryPoints(lastConfirmedEpoch, latestCommitableEpoch epoch.Index, prodChan chan *SolidEntryPoints) {
	go func() {
		for i := lastConfirmedEpoch; i <= latestCommitableEpoch; i++ {
			sepsPrefix := byteutils.ConcatBytes([]byte{database.PrefixSnapshot, prefixSolidEntryPoint}, i.Bytes())
			seps := make([]tangleold.BlockID, 0)

			m.baseStore.IterateKeys(sepsPrefix, func(key kvstore.Key) bool {
				var blkID tangleold.BlockID
				_, err := blkID.FromBytes(key)
				if err != nil {
					return false
				}
				seps = append(seps, blkID)

				return true
			})

			send := &SolidEntryPoints{
				EI:   i,
				Seps: seps,
			}
			prodChan <- send
		}

		close(prodChan)
	}()

	return
}
