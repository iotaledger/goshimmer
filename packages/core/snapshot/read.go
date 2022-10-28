package snapshot

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage/ledger"
	"github.com/iotaledger/goshimmer/packages/storage/tangle"
)

func ReadSnapshot(fileHandle *os.File, engine *engine.Engine) {
	// Settings
	{
		var settingsSize uint32
		binary.Read(fileHandle, binary.LittleEndian, &settingsSize)
		settingsBytes := make([]byte, settingsSize)
		binary.Read(fileHandle, binary.LittleEndian, settingsBytes)
		engine.Storage.Settings.FromBytes(settingsBytes)
	}

	// Committments
	{
		ProcessChunks(NewChunkedReader[commitment.Commitment](fileHandle), func(chunk []*commitment.Commitment) {
			for _, commitment := range chunk {
				if err := engine.Storage.StoreCommitment(commitment.Index(), commitment); err != nil {
					panic(err)
				}
			}
		})
	}

	engine.SnapshotCommitment = lo.PanicOnErr(engine.Storage.LoadCommitment(engine.Storage.LatestCommitment().Index()))
	if err := engine.Storage.SetChainID(engine.SnapshotCommitment.ID()); err != nil {
		panic(err)
	}

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[ledger.OutputWithMetadata](fileHandle),
			engine.Ledger.LoadOutputsWithMetadata,
			engine.ManaTracker.LoadOutputsWithMetadata,
			func(chunk []*ledger.OutputWithMetadata) {
				engine.Storage.Ledger.ImportUnspentOutputIDs(lo.Map(chunk, (*ledger.OutputWithMetadata).ID))
			},
		)
	}

	// Solid Entry Points
	{
		ProcessChunks(NewChunkedReader[models.Block](fileHandle), func(chunk []*models.Block) {
			for _, block := range chunk {
				block.DetermineID()
				if err := engine.Storage.Tangle.SolidEntryPointsStorage.Store(block); err != nil {
					panic(err)
				}
			}
		})
	}

	// Activity Log
	{
		var numEpochs uint32
		binary.Read(fileHandle, binary.LittleEndian, &numEpochs)

		for i := uint32(0); i < numEpochs; i++ {
			ProcessChunks(NewChunkedReader[identity.ID](fileHandle), func(chunk []*identity.ID) {
				for _, id := range chunk {
					if err := engine.Storage.Tangle.ActivityLogStorage.Store(&tangle.ActivityEntry{
						Index: epoch.Index(i),
						ID:    *id,
					}); err != nil {
						panic(err)
					}
				}
			})
		}
	}

	// Epoch Diffs -- must be in reverse order to rollback the Ledger
	{
		var numEpochs uint32
		binary.Read(fileHandle, binary.LittleEndian, &numEpochs)

		for i := uint32(1); i <= numEpochs; i++ {
			var epochIndex epoch.Index
			binary.Read(fileHandle, binary.LittleEndian, &epochIndex)

			diff := ledger.NewMemoryStateDiff()

			// Created
			ProcessChunks(NewChunkedReader[ledger.OutputWithMetadata](fileHandle),
				func(createdChunk []*ledger.OutputWithMetadata) {
					diff.ApplyCreatedOutputs(createdChunk)
				},
				engine.Ledger.ApplySpentDiff,
			)

			// Spent
			ProcessChunks(NewChunkedReader[ledger.OutputWithMetadata](fileHandle),
				func(spentChunk []*ledger.OutputWithMetadata) {
					diff.ApplyDeletedOutputs(spentChunk)
				},
				engine.Ledger.ApplyCreatedDiff,
			)

			engine.Storage.Ledger.RollbackEpochStateDiff(engine.Storage.LatestStateMutationEpoch()-epoch.Index(i), diff)
		}
	}
}

func ProcessChunks[A any, B constraints.MarshalablePtr[A]](chunkedReader *ChunkedReader[A, B], chunkConsumers ...func([]B)) {
	for !chunkedReader.IsFinished() {
		chunk, err := chunkedReader.ReadChunk()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		for _, chunkConsumer := range chunkConsumers {
			chunkConsumer(chunk)
		}
	}
}
