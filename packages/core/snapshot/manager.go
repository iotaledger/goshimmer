package snapshot

import (
	"errors"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/types"
)

// Manager is the snapshot manager.
type Manager struct {
	sync.RWMutex

	notarizationMgr *notarization.Manager
	seps            *shrinkingmap.ShrinkingMap[epoch.Index, map[tangleold.BlockID]types.Empty]
	snapshotDepth   int
}

// NewManager creates and returns a new snapshot manager.
func NewManager(nmgr *notarization.Manager, depth int) (new *Manager) {
	new = &Manager{
		notarizationMgr: nmgr,
		seps:            shrinkingmap.New[epoch.Index, map[tangleold.BlockID]types.Empty](),
		snapshotDepth:   depth,
	}

	return
}

// CreateSnapshot creates a snapshot file from node with a given name.
func (m *Manager) CreateSnapshot(snapshotFileName string) (header *ledger.SnapshotHeader, err error) {
	// lock the entire solid entry points storage until the snapshot is created.
	m.RLock()
	defer m.RUnlock()

	fullEpochIndex, ecRecord, err := m.notarizationMgr.StartSnapshot()
	defer m.notarizationMgr.EndSnapshot()

	headerProd := func() (header *ledger.SnapshotHeader, err error) {
		header = &ledger.SnapshotHeader{
			FullEpochIndex: fullEpochIndex,
			DiffEpochIndex: ecRecord.EI(),
			LatestECRecord: ecRecord,
		}
		return header, nil
	}

	sepsProd := NewSolidEntryPointsProducer(fullEpochIndex, ecRecord.EI(), m)
	outputWithMetadataProd := NewLedgerUTXOStatesProducer(m.notarizationMgr)
	epochDiffsProd := NewEpochDiffsProducer(fullEpochIndex, ecRecord.EI(), m.notarizationMgr)
	activityProducer := NewActivityLogProducer(m.notarizationMgr, ecRecord.EI())

	header, err = CreateSnapshot(snapshotFileName, headerProd, sepsProd, outputWithMetadataProd, epochDiffsProd, activityProducer)

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
	m.seps.Set(seps.EI, sep)
}

// AdvanceSolidEntryPoints remove seps of old epoch when confirmed epoch advanced.
func (m *Manager) AdvanceSolidEntryPoints(ei epoch.Index) {
	m.Lock()
	defer m.Unlock()

	// preserve seps of last confirmed epoch until now
	m.seps.Delete(ei - epoch.Index(m.snapshotDepth) - 1)
}

// InsertSolidEntryPoint inserts a solid entry point to the seps map.
func (m *Manager) InsertSolidEntryPoint(id tangleold.BlockID) {
	m.Lock()
	defer m.Unlock()

	sep, ok := m.seps.Get(id.EpochIndex)
	if !ok {
		sep = make(map[tangleold.BlockID]types.Empty)
	}

	sep[id] = types.Void
	m.seps.Set(id.EpochIndex, sep)
}

// RemoveSolidEntryPoint removes a solid entry points from the map.
func (m *Manager) RemoveSolidEntryPoint(b *tangleold.Block) (err error) {
	m.Lock()
	defer m.Unlock()

	epochSeps, exists := m.seps.Get(b.ID().EpochIndex)
	if !exists {
		return errors.New("solid entry point of the epoch does not exist")
	}

	delete(epochSeps, b.ID())

	return
}

// snapshotSolidEntryPoints snapshots seps within given epochs.
func (m *Manager) snapshotSolidEntryPoints(lastConfirmedEpoch, latestCommitableEpoch epoch.Index, prodChan chan *SolidEntryPoints, stopChan chan struct{}) {
	go func() {
		for i := lastConfirmedEpoch; i <= latestCommitableEpoch; i++ {
			seps := make([]tangleold.BlockID, 0)

			epochSeps, _ := m.seps.Get(i)
			for blkID := range epochSeps {
				seps = append(seps, blkID)
			}

			send := &SolidEntryPoints{
				EI:   i,
				Seps: seps,
			}
			prodChan <- send
		}

		close(stopChan)
	}()

	return
}
