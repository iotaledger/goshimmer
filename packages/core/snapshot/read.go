package snapshot

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	storageModels "github.com/iotaledger/goshimmer/packages/storage/models"
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
				if err := engine.Storage.Commitments.Store(commitment.Index(), commitment); err != nil {
					panic(err)
				}
			}
		})
	}

	if err := engine.Storage.Settings.SetChainID(engine.Storage.Settings.LatestCommitment().ID()); err != nil {
		panic(err)
	}

	// We need to set the genesis time before we add the activity log as otherwise the calculation is based on the empty time value.
	engine.Clock.SetAcceptedTime(engine.Storage.Settings.LatestCommitment().Index().EndTime())
	engine.Clock.SetConfirmedTime(engine.Storage.Settings.LatestCommitment().Index().EndTime())

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[storageModels.OutputWithMetadata](fileHandle),
			engine.Ledger.ImportOutputs,
			engine.ManaTracker.ImportOutputs,
			// This will import into all the consumers too: sybilprotection and ledgerState.unspentOutputIDs
			engine.LedgerState.ImportOutputs,
		)
	}

	// Solid Entry Points
	{
		ProcessChunks(NewChunkedReader[models.BlockID](fileHandle), func(chunk []*models.BlockID) {
			for _, blockID := range chunk {
				if err := engine.Storage.RootBlocks.Store(*blockID); err != nil {
					panic(err)
				}
			}
		})
	}

	// Activity Log
	{
		ProcessChunks(NewChunkedReader[identity.ID](fileHandle), func(chunk []*identity.ID) {
			for _, id := range chunk {
				if err := engine.Storage.Attestors.Store(engine.Storage.Settings.LatestCommitment().Index(), *id); err != nil {
					panic(err)
				}
			}
		})
	}

	// Epoch Diffs -- must be in reverse order to rollback the Ledger
	{
		var numEpochs uint32
		binary.Read(fileHandle, binary.LittleEndian, &numEpochs)

		for i := uint32(1); i <= numEpochs; i++ {
			var epochIndex epoch.Index
			binary.Read(fileHandle, binary.LittleEndian, &epochIndex)

			// Created
			ProcessChunks(NewChunkedReader[storageModels.OutputWithMetadata](fileHandle),
				engine.Ledger.ApplySpentDiff,
				func(createdChunk []*storageModels.OutputWithMetadata) {
					engine.ManaTracker.RollbackOutputs(createdChunk, true)
					for _, createdOutput := range createdChunk {
						engine.Storage.LedgerStateDiffs.StoreSpentOutput(createdOutput)
					}
				},
			)

			// Spent
			ProcessChunks(NewChunkedReader[storageModels.OutputWithMetadata](fileHandle),
				engine.Ledger.ApplyCreatedDiff,
				func(spentChunk []*storageModels.OutputWithMetadata) {
					engine.ManaTracker.RollbackOutputs(spentChunk, false)
					for _, createdOutput := range spentChunk {
						engine.Storage.LedgerStateDiffs.StoreCreatedOutput(createdOutput)
					}
				},
			)

			engine.LedgerState.ApplyStateDiff(epochIndex)
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
