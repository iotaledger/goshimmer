package snapshot

import (
	"errors"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"
)

const (
	prefixSolidEntryPoint byte = iota
)

// Manager is the snapshot manager.
type Manager struct {
	sync.RWMutex

	notarizationMgr *notarization.Manager
	seps            map[epoch.Index]map[tangleold.BlockID]types.Empty
	snapshotDepth   int
}

// NewManager creates and returns a new snapshot manager.
func NewManager(store kvstore.KVStore, nmgr *notarization.Manager, depth int) (new *Manager) {
	new = &Manager{
		notarizationMgr: nmgr,
		seps:            make(map[epoch.Index]map[tangleold.BlockID]types.Empty),
		snapshotDepth:   depth,
	}

	return
}

// CreateSnapshot creates a snapshot file from node with a given name.
func (m *Manager) CreateSnapshot(snapshotFileName string) (header *ledger.SnapshotHeader, err error) {
	lastConfirmedEpoch, ecRecord, err := m.notarizationMgr.StartSnapshot()
	defer m.notarizationMgr.EndSnapshot()

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

	m.Lock()
	defer m.Unlock()

	sep := make(map[tangleold.BlockID]types.Empty)
	for _, b := range seps.Seps {
		sep[b] = types.Void
	}
	m.seps[seps.EI] = sep
}

// AdvanceSolidEntryPoints remove seps of old epoch when confirmed epoch advanced.
func (m *Manager) AdvanceSolidEntryPoints(ei epoch.Index) {
	m.Lock()
	defer m.Unlock()

	// preserve seps of last confirmed epoch until now
	delete(m.seps, ei-epoch.Index(m.snapshotDepth)-1)
}

// InsertSolidEntryPoint inserts a solid entry point to the seps map.
func (m *Manager) InsertSolidEntryPoint(id tangleold.BlockID) {
	m.Lock()
	defer m.Unlock()

	sep, ok := m.seps[id.EpochIndex]
	if !ok {
		sep = make(map[tangleold.BlockID]types.Empty)
	}

	sep[id] = types.Void
	m.seps[id.EpochIndex] = sep
}

// RemoveSolidEntryPoint removes a solid entry points from the map.
func (m *Manager) RemoveSolidEntryPoint(b *tangleold.Block) (err error) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.seps[b.EI()]; !ok {
		return errors.New("solid entry point of the epoch does not exist")
	}

	delete(m.seps[b.EI()], b.ID())

	return
}

// SnapshotSolidEntryPoints snapshots seps within given epochs.
func (m *Manager) SnapshotSolidEntryPoints(lastConfirmedEpoch, latestCommitableEpoch epoch.Index, prodChan chan *SolidEntryPoints) {
	go func() {
		for i := lastConfirmedEpoch; i <= latestCommitableEpoch; i++ {
			seps := make([]tangleold.BlockID, 0)

			for blkID := range m.seps[i] {
				seps = append(seps, blkID)
			}

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
