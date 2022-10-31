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

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[storageModels.OutputWithMetadata](fileHandle),
			engine.Ledger.LoadOutputsWithMetadata,
			engine.ManaTracker.LoadOutputsWithMetadata,
			func(chunk []*storageModels.OutputWithMetadata) {
				engine.Storage.UnspentOutputIDs.Import(lo.Map(chunk, (*storageModels.OutputWithMetadata).ID))
			},
		)
	}

	// Solid Entry Points
	{
		ProcessChunks(NewChunkedReader[models.Block](fileHandle), func(chunk []*models.Block) {
			for _, block := range chunk {
				block.DetermineID()
				if err := engine.Storage.SolidEntryPoints.Store(block); err != nil {
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
					if err := engine.Storage.ActiveNodes.Store(epoch.Index(i), *id); err != nil {
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

			diff := storageModels.NewMemoryStateDiff()

			// Created
			ProcessChunks(NewChunkedReader[storageModels.OutputWithMetadata](fileHandle),
				func(createdChunk []*storageModels.OutputWithMetadata) {
					diff.ApplyCreatedOutputs(createdChunk)
				},
				engine.Ledger.ApplySpentDiff,
			)

			// Spent
			ProcessChunks(NewChunkedReader[storageModels.OutputWithMetadata](fileHandle),
				func(spentChunk []*storageModels.OutputWithMetadata) {
					diff.ApplyDeletedOutputs(spentChunk)
				},
				engine.Ledger.ApplyCreatedDiff,
			)

			engine.Storage.RollbackStateDiff(engine.Storage.Settings.LatestStateMutationEpoch()-epoch.Index(i), diff)
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
