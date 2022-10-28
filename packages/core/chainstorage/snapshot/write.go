package snapshot

import (
	"encoding/binary"
	"os"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"
)

func WriteSnapshot(filePath string, engine *engine.Engine, depth int) {
	fileHandle, err := os.Create(filePath)
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
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch+1))
		// Commitment size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&commitment.Commitment{}).Bytes()))))
		for epochIndex := epoch.Index(0); epochIndex <= snapshotEpoch; epochIndex++ {
			binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(lo.PanicOnErr(engine.ChainStorage.Commitments.Get(int(epochIndex))).Bytes()))
		}
	}

	var outputWithMetadataSize uint32

	// Ledgerstate
	{
		var outputCount uint32
		var dummyOutput utxo.Output
		engine.Ledger.Storage.ForEachOutputID(func(outputID utxo.OutputID) bool {
			engine.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
				if (outputMetadata.ConfirmationState() == confirmation.Accepted || outputMetadata.ConfirmationState() == confirmation.Confirmed) &&
					!outputMetadata.IsSpent() {
					outputCount++
				}
			})

			if dummyOutput == nil {
				engine.Ledger.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
					dummyOutput = output
				})
			}

			return true
		})

		// TODO: seek back to this location instead of scanning the collection twice
		// Output count
		binary.Write(fileHandle, binary.LittleEndian, outputCount)

		// OutputWithMetadata size
		dummyOutputWithMetadata := chainstorage.NewOutputWithMetadata(0, dummyOutput.ID(), dummyOutput, time.Unix(epoch.GenesisTime, 0), identity.ID{}, identity.ID{})
		outputWithMetadataSize = uint32(len(lo.PanicOnErr(dummyOutputWithMetadata.Bytes())))
		binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)

		engine.Ledger.Storage.ForEachOutputID(func(outputID utxo.OutputID) bool {
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
			return true
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
		dummyBlock := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)))
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr(dummyBlock.Bytes()))))

		for epochIndex := startEpoch; epochIndex <= snapshotEpoch; epochIndex++ {
			engine.ChainStorage.SolidEntryPointsStorage.Stream(epochIndex, func(block *models.Block) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(block.Bytes()))
			})
		}
	}

	// Activity Log
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-startEpoch+1))

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

		for epochIndex := snapshotEpoch; epochIndex >= startEpoch; epochIndex-- {
			// Epoch Index
			binary.Write(fileHandle, binary.LittleEndian, epochIndex)

			var createdCount uint32
			engine.ChainStorage.DiffStorage.StreamCreated(epochIndex, func(_ *chainstorage.OutputWithMetadata) {
				createdCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Created count
			binary.Write(fileHandle, binary.LittleEndian, createdCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			engine.ChainStorage.DiffStorage.StreamCreated(epochIndex, func(createdWithMetadata *chainstorage.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(createdWithMetadata.Bytes()))
			})

			var spentCount uint32
			engine.ChainStorage.DiffStorage.StreamSpent(epochIndex, func(_ *chainstorage.OutputWithMetadata) {
				spentCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Spent count
			binary.Write(fileHandle, binary.LittleEndian, spentCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			engine.ChainStorage.DiffStorage.StreamSpent(epochIndex, func(spentWithMetadata *chainstorage.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(spentWithMetadata.Bytes()))
			})
		}
	}
}
