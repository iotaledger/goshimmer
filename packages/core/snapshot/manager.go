package snapshot

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// Manager is the snapshot manager.
type Manager struct {
	sync.RWMutex

	notarizationMgr *notarization.Manager
	snapshotDepth   int
}

// NewManager creates and returns a new snapshot manager.
func NewManager(nmgr *notarization.Manager, depth int) (new *Manager) {
	new = &Manager{
		notarizationMgr: nmgr,
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
			DiffEpochIndex: ecRecord.Commitment().Index(),
			LatestECRecord: ecRecord.Commitment(),
		}
		return header, nil
	}

	sepsProd := NewSolidEntryPointsProducer(fullEpochIndex, ecRecord.Commitment().Index(), m)
	outputWithMetadataProd := NewLedgerUTXOStatesProducer(m.notarizationMgr)
	epochDiffsProd := NewEpochDiffsProducer(fullEpochIndex, ecRecord.Commitment().Index(), m.notarizationMgr)
	activityProducer := NewActivityLogProducer(m.notarizationMgr, ecRecord.Commitment().Index())

	header, err = CreateSnapshot(snapshotFileName, headerProd, sepsProd, outputWithMetadataProd, epochDiffsProd, activityProducer)

	return
}

// snapshotSolidEntryPoints snapshots seps within given epochs.
func (m *Manager) snapshotSolidEntryPoints(lastConfirmedEpoch, latestCommitableEpoch epoch.Index, prodChan chan *SolidEntryPoints, stopChan chan struct{}) {
	go func() {
		for i := lastConfirmedEpoch; i <= latestCommitableEpoch; i++ {
			seps := make([]models.BlockID, 0)

			epochSeps, _ := m.entryPoints.Get(i)
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
