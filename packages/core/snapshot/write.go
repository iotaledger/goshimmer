package snapshot

import (
	"encoding/binary"
	"os"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
)

func WriteSnapshot(filePath string, engineInstance *engine.Engine, depth epoch.Index) {
	fileHandle, err := os.Create(filePath)
	defer fileHandle.Close()

	if err != nil {
		panic(err)
	}

	currentEpoch := engineInstance.Storage.Settings.LatestCommitment().Index()
	targetEpoch := currentEpoch - depth

	if err := engineInstance.Storage.Settings.Export(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Commitments.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	} else if err := engineInstance.EvictionState.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	} else if err := engineInstance.LedgerState.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	}

	var outputWithMetadataSize uint32

	// Activity Log
	{
		// Activity Log count
		binary.Write(fileHandle, binary.LittleEndian, uint32(engineInstance.Storage.Attestors.LoadAll(currentEpoch).Size()))
		// Activity Log size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&identity.ID{}).Bytes()))))
		engineInstance.Storage.Attestors.Stream(currentEpoch, func(id identity.ID) {
			binary.Write(fileHandle, binary.LittleEndian, id)
		})
	}

	// Epoch Diffs -- must be in reverse order to allow Ledger rollback
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(currentEpoch-engineInstance.Storage.Settings.LatestStateMutationEpoch()))

		for epochIndex := engineInstance.Storage.Settings.LatestStateMutationEpoch(); epochIndex >= currentEpoch; epochIndex-- {

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
