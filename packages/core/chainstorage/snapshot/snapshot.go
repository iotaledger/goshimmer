package snapshot

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
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

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[chainstorage.OutputWithMetadata](fileHandle),
			engine.Ledger.LoadOutputsWithMetadata, engine.ManaTracker.LoadOutputsWithMetadata, engine.NotarizationManager.LoadOutputsWithMetadata)
	}

	// Solid Entry Points
	{
		ProcessChunks(NewChunkedReader[models.Block](fileHandle), func(chunk []*models.Block) {
			for _, block := range chunk {
				engine.ChainStorage.SolidEntryPointsStorage.Store(block)
			}
		})
	}

	// Activity Log
	{
		var numEpochs uint32
		binary.Read(fileHandle, binary.LittleEndian, &numEpochs)

		for i := uint32(0); i < numEpochs; i++ {
			ProcessChunks(NewChunkedReader[chainstorage.ActivityEntry](fileHandle), func(chunk []*chainstorage.ActivityEntry) {
				for _, activityEntry := range chunk {
					engine.ChainStorage.ActivityLogStorage.Store(activityEntry)
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

			// Created
			ProcessChunks(NewChunkedReader[chainstorage.OutputWithMetadata](fileHandle),
				func(createdChunk []*chainstorage.OutputWithMetadata) {
					for _, createdOutputWithMetadata := range createdChunk {
						engine.ChainStorage.DiffStorage.StoreCreated(createdOutputWithMetadata)
					}
					engine.NotarizationManager.RollbackOutputs(createdChunk, true)
					engine.ManaTracker.RollbackOutputs(epochIndex, createdChunk, true)
				},
				engine.Ledger.ApplySpentDiff,
			)

			// Spent
			ProcessChunks(NewChunkedReader[chainstorage.OutputWithMetadata](fileHandle),
				func(spentChunk []*chainstorage.OutputWithMetadata) {
					for _, spentOutputWithMetadata := range spentChunk {
						engine.ChainStorage.DiffStorage.StoreSpent(spentOutputWithMetadata)
					}
					engine.NotarizationManager.RollbackOutputs(spentChunk, false)
					engine.ManaTracker.RollbackOutputs(epochIndex, spentChunk, false)
				},
				engine.Ledger.ApplyCreatedDiff,
			)
		}
	}
}

func WriteSnapshot(filePath string, engine *engine.Engine, depth int) {
	fileHandle, err := os.Open(filePath)
	defer fileHandle.Close()

	if err != nil {
		panic(err)
	}

	snapshotEpoch := engine.ChainStorage.LatestCommittedEpoch()
	startEpoch := snapshotEpoch - epoch.Index(depth)

	// Settings
	{
		settingsBytes := lo.PanicOnErr(engine.ChainStorage.Settings.Bytes())
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(settingsBytes)))
		binary.Write(fileHandle, binary.LittleEndian, settingsBytes)
	}

	// Committments
	{
		// Commitments count, we dump all commitments from Genesis
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch))
		// Commitment size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&commitment.Commitment{}).Bytes()))))
		for epochIndex := epoch.Index(0); epochIndex <= snapshotEpoch; epochIndex++ {
			binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(lo.PanicOnErr(engine.ChainStorage.Commitments.Get(int(epochIndex))).Bytes()))
		}
	}

	// Ledgerstate
	{
		var outputCount uint32
		engine.Ledger.Storage.ForEachOutputID(func(outputID utxo.OutputID) {
			engine.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
				if outputMetadata.ConfirmationState() == confirmation.Accepted && !outputMetadata.IsSpent() {
					outputCount++
				}
			})
		})

		// Output count
		binary.Write(fileHandle, binary.LittleEndian, outputCount)
		// OutputWithMetadata size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&chainstorage.OutputWithMetadata{}).Bytes()))))

		engine.Ledger.Storage.ForEachOutputID(func(outputID utxo.OutputID) {
			engine.Ledger.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
				engine.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
					outputWithMetadata := chainstorage.NewOutputWithMetadata(
						epoch.IndexFromTime(outputMetadata.CreationTime()),
						outputID,
						output,
						outputMetadata.CreationTime(),
						outputMetadata.ConsensusManaPledgeID(),
						outputMetadata.AccessManaPledgeID(),
					)
					binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(outputWithMetadata.Bytes()))
				})
			})
		})
	}

	// Solid Entry Points
	{
		var solidEntryPointsCount uint32
		for epochIndex := startEpoch; epochIndex <= snapshotEpoch; epochIndex++ {
			solidEntryPointsCount += uint32(engine.ChainStorage.SolidEntryPointsStorage.GetAll(epochIndex).Size())
		}

		// Solid Entry Points count
		binary.Write(fileHandle, binary.LittleEndian, solidEntryPointsCount)
		// Solid Entry Point size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&models.Block{}).Bytes()))))

		for epochIndex := startEpoch; epochIndex <= snapshotEpoch; epochIndex++ {
			engine.ChainStorage.SolidEntryPointsStorage.Stream(epochIndex, func(block *models.Block) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(block.Bytes()))
			})
		}
	}

	// Activity Log
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-startEpoch))

		for epochIndex := startEpoch; epochIndex <= snapshotEpoch; epochIndex++ {
			// Activity Log count
			binary.Write(fileHandle, binary.LittleEndian, uint32(engine.ChainStorage.ActivityLogStorage.GetAll(epochIndex).Size()))
			// Activity Log size
			binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&identity.ID{}).Bytes()))))
			engine.ChainStorage.ActivityLogStorage.Stream(epochIndex, func(id identity.ID) {
				binary.Write(fileHandle, binary.LittleEndian, id)
			})
		}
	}

	// Epoch Diffs -- must be in reverse order to allow Ledger rollback
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-startEpoch))

		for epochIndex := snapshotEpoch; epochIndex >= startEpoch; epochIndex++ {
			// Epoch Index
			binary.Write(fileHandle, binary.LittleEndian, epochIndex)

			var createdCount uint32
			engine.ChainStorage.DiffStorage.StreamCreated(epochIndex, func(_ *chainstorage.OutputWithMetadata) {
				createdCount++
			})

			// Created count
			binary.Write(fileHandle, binary.LittleEndian, createdCount)
			engine.ChainStorage.DiffStorage.StreamCreated(epochIndex, func(createdWithMetadata *chainstorage.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(createdWithMetadata.Bytes()))
			})

			var spentCount uint32
			engine.ChainStorage.DiffStorage.StreamSpent(epochIndex, func(_ *chainstorage.OutputWithMetadata) {
				spentCount++
			})

			// Spent count
			binary.Write(fileHandle, binary.LittleEndian, spentCount)
			engine.ChainStorage.DiffStorage.StreamSpent(epochIndex, func(spentWithMetadata *chainstorage.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(spentWithMetadata.Bytes()))
			})
		}
	}
}

func ProcessChunks[A any, B constraints.MarshalablePtr[A]](chunkedReader *ChunkedReader[A, B], chunkConsumers ...func([]B)) {
	for {
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
