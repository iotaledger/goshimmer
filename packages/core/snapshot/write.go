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
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	storageModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

func WriteSnapshot(filePath string, s *storage.Storage, l *ledger.Ledger, depth int) {
	fileHandle, err := os.Create(filePath)
	defer fileHandle.Close()

	if err != nil {
		panic(err)
	}

	snapshotEpoch := s.Settings.LatestCommitment().Index()
	snapshotStart := snapshotEpoch - epoch.Index(depth)

	// Settings
	{
		settingsBytes := lo.PanicOnErr(s.Settings.Bytes())
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
			binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(lo.PanicOnErr(s.Commitments.Load(epochIndex)).Bytes()))
		}
	}

	var outputWithMetadataSize uint32

	// Ledgerstate
	{
		var outputCount uint32
		var dummyOutput utxo.Output
		l.Storage.ForEachOutputID(func(outputID utxo.OutputID) bool {
			l.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
				if (outputMetadata.ConfirmationState() == confirmation.Accepted || outputMetadata.ConfirmationState() == confirmation.Confirmed) &&
					!outputMetadata.IsSpent() {
					outputCount++
				}
			})

			if dummyOutput == nil {
				l.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
					dummyOutput = output
				})
			}

			return true
		})

		// TODO: seek back to this location instead of scanning the collection twice
		// Output count
		binary.Write(fileHandle, binary.LittleEndian, outputCount)

		// OutputWithMetadata size
		dummyOutputWithMetadata := storageModels.NewOutputWithMetadata(0, dummyOutput.ID(), dummyOutput, time.Unix(epoch.GenesisTime, 0), identity.ID{}, identity.ID{})
		outputWithMetadataSize = uint32(len(lo.PanicOnErr(dummyOutputWithMetadata.Bytes())))
		binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)

		l.Storage.ForEachOutputID(func(outputID utxo.OutputID) bool {
			l.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
				l.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
					outputWithMetadata := storageModels.NewOutputWithMetadata(
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
			solidEntryPointsCount += uint32(s.EntryPoints.LoadAll(epochIndex).Size())
		}

		// Solid Entry Points count
		binary.Write(fileHandle, binary.LittleEndian, solidEntryPointsCount)
		// Solid Entry Point size
		dummyBlock := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)))
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr(dummyBlock.Bytes()))))

		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			s.EntryPoints.Stream(epochIndex, func(blockID models.BlockID) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(blockID.Bytes()))
			})
		}
	}

	// Activity Log
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-snapshotStart+1))

		for epochIndex := snapshotStart; epochIndex <= snapshotEpoch; epochIndex++ {
			// Activity Log count
			binary.Write(fileHandle, binary.LittleEndian, uint32(s.ActiveNodes.LoadAll(epochIndex).Size()))
			// Activity Log size
			binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&identity.ID{}).Bytes()))))
			s.ActiveNodes.Stream(epochIndex, func(id identity.ID) {
				binary.Write(fileHandle, binary.LittleEndian, id)
			})
		}
	}

	// Epoch Diffs -- must be in reverse order to allow Ledger rollback
	{
		// Number of epochs
		binary.Write(fileHandle, binary.LittleEndian, uint32(snapshotEpoch-s.Settings.LatestStateMutationEpoch()))

		for epochIndex := s.Settings.LatestStateMutationEpoch(); epochIndex >= snapshotEpoch; epochIndex-- {

			// Epoch Index
			binary.Write(fileHandle, binary.LittleEndian, epochIndex)

			var createdCount uint32
			s.LedgerStateDiffs.StreamCreatedOutputs(epochIndex, func(_ *storageModels.OutputWithMetadata) {
				createdCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Created count
			binary.Write(fileHandle, binary.LittleEndian, createdCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			s.LedgerStateDiffs.StreamCreatedOutputs(epochIndex, func(createdWithMetadata *storageModels.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(createdWithMetadata.Bytes()))
			})

			var spentCount uint32
			s.LedgerStateDiffs.StreamSpentOutputs(epochIndex, func(_ *storageModels.OutputWithMetadata) {
				spentCount++
			})

			// TODO: seek back to this location instead of scanning the collection twice
			// Spent count
			binary.Write(fileHandle, binary.LittleEndian, spentCount)
			// OutputWithMetadata size
			binary.Write(fileHandle, binary.LittleEndian, outputWithMetadataSize)
			s.LedgerStateDiffs.StreamSpentOutputs(epochIndex, func(spentWithMetadata *storageModels.OutputWithMetadata) {
				binary.Write(fileHandle, binary.LittleEndian, lo.PanicOnErr(spentWithMetadata.Bytes()))
			})
		}
	}
}
