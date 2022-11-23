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
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func ReadSnapshot(fileHandle *os.File, engineInstance *engine.Engine) {
	// Settings
	{
		var settingsSize uint32
		binary.Read(fileHandle, binary.LittleEndian, &settingsSize)
		settingsBytes := make([]byte, settingsSize)
		binary.Read(fileHandle, binary.LittleEndian, settingsBytes)
		engineInstance.Storage.Settings.FromBytes(settingsBytes)
	}

	// Committments
	{
		ProcessChunks(NewChunkedReader[commitment.Commitment](fileHandle), func(chunk []*commitment.Commitment) {
			for _, commitment := range chunk {
				if err := engineInstance.Storage.Commitments.Store(commitment.Index(), commitment); err != nil {
					panic(err)
				}
			}
		})
	}

	if err := engineInstance.Storage.Settings.SetChainID(engineInstance.Storage.Settings.LatestCommitment().ID()); err != nil {
		panic(err)
	}

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[ledgerstate.OutputWithMetadata](fileHandle),
			engineInstance.Ledger.ImportOutputs,
			engineInstance.ManaTracker.ImportOutputs,
			// This will import into all the consumers too: sybilprotection and ledgerState.unspentOutputIDs
			engineInstance.LedgerState.ImportOutputs,
		)
	}

	// Solid Entry Points
	{
		ProcessChunks(NewChunkedReader[models.BlockID](fileHandle), func(chunk []*models.BlockID) {
			for _, blockID := range chunk {
				if err := engineInstance.Storage.RootBlocks.Store(*blockID); err != nil {
					panic(err)
				}
			}
		})
	}

	// Activity Log
	{
		ProcessChunks(NewChunkedReader[identity.ID](fileHandle), func(chunk []*identity.ID) {
			for _, id := range chunk {
				if err := engineInstance.Storage.Attestors.Store(engineInstance.Storage.Settings.LatestCommitment().Index(), *id); err != nil {
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
			ProcessChunks(NewChunkedReader[ledgerstate.OutputWithMetadata](fileHandle),
				engineInstance.Ledger.ApplySpentDiff,
				func(createdChunk []*ledgerstate.OutputWithMetadata) {
					engineInstance.ManaTracker.RollbackOutputs(createdChunk, true)
					for _, createdOutput := range createdChunk {
						engineInstance.LedgerState.StateDiffs.StoreSpentOutput(createdOutput)
					}
				},
			)

			// Spent
			ProcessChunks(NewChunkedReader[ledgerstate.OutputWithMetadata](fileHandle),
				engineInstance.Ledger.ApplyCreatedDiff,
				func(spentChunk []*ledgerstate.OutputWithMetadata) {
					engineInstance.ManaTracker.RollbackOutputs(spentChunk, false)
					for _, createdOutput := range spentChunk {
						engineInstance.LedgerState.StateDiffs.StoreCreatedOutput(createdOutput)
					}
				},
			)

			engineInstance.LedgerState.ApplyStateDiff(epochIndex)
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
