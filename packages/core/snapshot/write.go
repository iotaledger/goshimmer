package snapshot

import (
	"encoding/binary"
	"os"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	ledgerStorage "github.com/iotaledger/goshimmer/packages/storage/ledger"
)

func WriteSnapshot(filePath string, engine *engine.Engine, depth int) {
	fileHandle, err := os.Create(filePath)
	defer fileHandle.Close()

	if err != nil {
		panic(err)
	}

	snapshotEpoch := engine.Storage.LatestCommitment().Index()
	snapshotStart := snapshotEpoch - epoch.Index(depth)

	// Settings
	{
		settingsBytes := lo.PanicOnErr(engine.Storage.Settings.Bytes())
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
			binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(lo.PanicOnErr(engine.Storage.LoadCommitment(epochIndex)).Bytes()))
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
		dummyOutputWithMetadata := ledgerStorage.NewOutputWithMetadata(0, dummyOutput.ID(), dummyOutput, time.Unix(epoch.GenesisTime, 0), identity.ID{}, identity.ID{})
		outputWithMetadataSize = uint32(len(lo.PanicOnErr(dummyOutputWithMetadata.Bytes())))
		binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)

		engine.Ledger.Storage.ForEachOutputID(func(outputID utxo.OutputID) bool {
			engine.Ledger.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
				engine.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
					outputWithMetadata := ledgerStorage.NewOutputWithMetadata(
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
		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			solidEntryPointsCount += uint32(engine.Storage.Tangle.SolidEntryPointsStorage.GetAll(epochIndex).Size())
		}

		// Solid Entry Points count
		binary.Write(fileHandle, binary.LittleEndian, solidEntryPointsCount)
		// Solid Entry Point size
		dummyBlock := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)))
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr(dummyBlock.Bytes()))))

		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			engine.Storage.Tangle.SolidEntryPointsStorage.Stream(epochIndex, func(block *models.Block) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(block.Bytes()))
			})
		}
	}

	// Activity Log
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-snapshotStart+1))

		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			// Activity Log count
			binary.Write(fileHandle, binary.LittleEndian, uint32(engine.Storage.Tangle.ActivityLogStorage.GetAll(epochIndex).Size()))
			// Activity Log size
			binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&identity.ID{}).Bytes()))))
			engine.Storage.Tangle.ActivityLogStorage.Stream(epochIndex, func(id identity.ID) {
				binary.Write(fileHandle, binary.LittleEndian, id)
			})
		}
	}

	// Epoch Diffs -- must be in reverse order to allow Ledger rollback
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-engine.Storage.LatestStateMutationEpoch()))

		for epochIndex := engine.Storage.LatestStateMutationEpoch(); epochIndex >= snapshotEpoch; epochIndex-- {
			// Epoch Index
			binary.Write(fileHandle, binary.LittleEndian, epochIndex)

			var createdCount uint32
			engine.Storage.Ledger.LedgerStateDiffs.StreamCreated(epochIndex, func(_ *ledgerStorage.OutputWithMetadata) {
				createdCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Created count
			binary.Write(fileHandle, binary.LittleEndian, createdCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			engine.Storage.Ledger.LedgerStateDiffs.StreamCreated(epochIndex, func(createdWithMetadata *ledgerStorage.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(createdWithMetadata.Bytes()))
			})

			var spentCount uint32
			engine.Storage.Ledger.LedgerStateDiffs.StreamSpent(epochIndex, func(_ *ledgerStorage.OutputWithMetadata) {
				spentCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Spent count
			binary.Write(fileHandle, binary.LittleEndian, spentCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			engine.Storage.Ledger.LedgerStateDiffs.StreamSpent(epochIndex, func(spentWithMetadata *ledgerStorage.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(spentWithMetadata.Bytes()))
			})
		}
	}
}
