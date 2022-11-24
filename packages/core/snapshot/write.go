package snapshot

import (
	"encoding/binary"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func WriteSnapshot(filePath string, engineInstance *engine.Engine, depth int) {
	fileHandle, err := os.Create(filePath)
	defer fileHandle.Close()

	if err != nil {
		panic(err)
	}

	snapshotEpoch := engineInstance.Storage.Settings.LatestCommitment().Index()
	snapshotStart := snapshotEpoch - epoch.Index(depth)

	if err := engineInstance.Storage.Settings.WriteTo(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Commitments.WriteTo(fileHandle, engineInstance.Storage.Settings.LatestCommitment().Index()); err != nil {
		panic(err)
	} else if err := engineInstance.LedgerState.WriteTo(fileHandle); err != nil {
		panic(err)
	}

	var outputWithMetadataSize uint32

	// Solid Entry Points
	{
		var solidEntryPointsCount uint32
		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			solidEntryPointsCount += uint32(engineInstance.Storage.RootBlocks.LoadAll(epochIndex).Size())
		}

		// Solid Entry Points count
		binary.Write(fileHandle, binary.LittleEndian, solidEntryPointsCount)
		// Solid Entry Point size
		dummyBlock := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)))
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr(dummyBlock.Bytes()))))

		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			if err := engineInstance.Storage.RootBlocks.Stream(epochIndex, func(blockID models.BlockID) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(blockID.Bytes()))
			}); err != nil {
				panic(errors.Errorf("failed streaming root blocks for snaphot: %w", err))
			}
		}
	}

	// Activity Log
	{
		// Activity Log count
		binary.Write(fileHandle, binary.LittleEndian, uint32(engineInstance.Storage.Attestors.LoadAll(snapshotEpoch).Size()))
		// Activity Log size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&identity.ID{}).Bytes()))))
		engineInstance.Storage.Attestors.Stream(snapshotEpoch, func(id identity.ID) {
			binary.Write(fileHandle, binary.LittleEndian, id)
		})
	}

	// Epoch Diffs -- must be in reverse order to allow Ledger rollback
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-engineInstance.Storage.Settings.LatestStateMutationEpoch()))

		for epochIndex := engineInstance.Storage.Settings.LatestStateMutationEpoch(); epochIndex >= snapshotEpoch; epochIndex-- {

			// Epoch Index
			binary.Write(fileHandle, binary.LittleEndian, epochIndex)

			var createdCount uint32
			engineInstance.LedgerState.StateDiffs.StreamCreatedOutputs(epochIndex, func(_ *ledgerstate.OutputWithMetadata) {
				createdCount++
			})
			// TODO: seek back to this location instead of scanning the collection twice
			// Created count
			binary.Write(fileHandle, binary.LittleEndian, createdCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			engineInstance.LedgerState.StateDiffs.StreamCreatedOutputs(epochIndex, func(createdWithMetadata *ledgerstate.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(createdWithMetadata.Bytes()))
			})

			var spentCount uint32
			engineInstance.LedgerState.StateDiffs.StreamSpentOutputs(epochIndex, func(_ *ledgerstate.OutputWithMetadata) {
				spentCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Spent count
			binary.Write(fileHandle, binary.LittleEndian, spentCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			engineInstance.LedgerState.StateDiffs.StreamSpentOutputs(epochIndex, func(spentWithMetadata *ledgerstate.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(spentWithMetadata.Bytes()))
			})
		}
	}
}
