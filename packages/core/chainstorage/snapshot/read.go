package snapshot

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func ReadSnapshot(fileHandle *os.File, engine *engine.Engine) {
	// Settings
	{
		var settingsSize uint32
		binary.Read(fileHandle, binary.LittleEndian, &settingsSize)
		settingsBytes := make([]byte, settingsSize)
		binary.Read(fileHandle, binary.LittleEndian, settingsBytes)
		engine.ChainStorage.Settings.FromBytes(settingsBytes)
	}

	// Committments
	{
		ProcessChunks(NewChunkedReader[commitment.Commitment](fileHandle), func(chunk []*commitment.Commitment) {
			for _, commitment := range chunk {
				engine.ChainStorage.Commitments.Set(int(commitment.Index()), commitment)
			}
		})
	}

	engine.SnapshotCommitment = lo.PanicOnErr(engine.ChainStorage.Commitments.Get(int(engine.ChainStorage.LatestCommitment().Index())))
	engine.ChainStorage.SetChain(engine.SnapshotCommitment.ID())

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[chainstorage.OutputWithMetadata](fileHandle),
			engine.Ledger.LoadOutputsWithMetadata, engine.ManaTracker.LoadOutputsWithMetadata, engine.ChainStorage.State.Import)
	}

	// Solid Entry Points
	{
		ProcessChunks(NewChunkedReader[models.Block](fileHandle), func(chunk []*models.Block) {
			for _, block := range chunk {
				block.DetermineID()
				engine.ChainStorage.SolidEntryPointsStorage.Store(block)
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
					engine.ChainStorage.ActivityLogStorage.Store(&chainstorage.ActivityEntry{
						Index: epoch.Index(i),
						ID:    *id,
					})
				}
			})
		}
	}

	// Epoch Diffs -- must be in reverse order to rollback the Ledger
	{
		var numEpochs uint32
		binary.Read(fileHandle, binary.LittleEndian, &numEpochs)

		for i := uint32(0); i < numEpochs; i++ {
			var epochIndex epoch.Index
			binary.Read(fileHandle, binary.LittleEndian, &epochIndex)

			diff := chainstorage.NewStateDiff()

			// Created
			ProcessChunks(NewChunkedReader[chainstorage.OutputWithMetadata](fileHandle),
				func(createdChunk []*chainstorage.OutputWithMetadata) {
					diff.ApplyCreatedOutputs(createdChunk)

					for _, createdOutputWithMetadata := range createdChunk {
						engine.ChainStorage.DiffStorage.StoreCreated(createdOutputWithMetadata)
					}

					engine.ManaTracker.RollbackOutputs(epochIndex, createdChunk, true)
				},
				engine.Ledger.ApplySpentDiff,
			)

			// Spent
			ProcessChunks(NewChunkedReader[chainstorage.OutputWithMetadata](fileHandle),
				func(spentChunk []*chainstorage.OutputWithMetadata) {
					diff.ApplyDeletedOutputs(spentChunk)

					for _, spentOutputWithMetadata := range spentChunk {
						engine.ChainStorage.DiffStorage.StoreSpent(spentOutputWithMetadata)
					}

					engine.ManaTracker.RollbackOutputs(epochIndex, spentChunk, false)
				},
				engine.Ledger.ApplyCreatedDiff,
			)

			engine.ChainStorage.State.Apply(diff)
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
