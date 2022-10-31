package ledger

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/storage/models"
)

// LoadOutputsWithMetadata loads OutputWithMetadata from a snapshot file to the storage.
func (l *Ledger) LoadOutputsWithMetadata(outputsWithMetadata []*models.OutputWithMetadata) {
	for _, outputWithMetadata := range outputsWithMetadata {
		newOutputMetadata := NewOutputMetadata(outputWithMetadata.ID())
		newOutputMetadata.SetAccessManaPledgeID(outputWithMetadata.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(outputWithMetadata.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		l.Storage.outputStorage.Store(outputWithMetadata.Output()).Release()
		l.Storage.outputMetadataStorage.Store(newOutputMetadata).Release()

		l.Events.OutputCreated.Trigger(outputWithMetadata.ID())
	}
}

// ApplySpentDiff applies the spent output to the Ledgerstate.
func (l *Ledger) ApplySpentDiff(spentOutputs []*models.OutputWithMetadata) {
	for _, spent := range spentOutputs {
		l.Storage.outputStorage.Delete(lo.PanicOnErr(spent.ID().Bytes()))
		l.Storage.outputMetadataStorage.Delete(lo.PanicOnErr(spent.ID().Bytes()))

		l.Events.OutputSpent.Trigger(spent.ID())
	}
}

// ApplyCreatedDiff applies the created output to the Ledgerstate.
func (l *Ledger) ApplyCreatedDiff(createdOutputs []*models.OutputWithMetadata) {
	for _, created := range createdOutputs {
		outputMetadata := NewOutputMetadata(created.ID())
		outputMetadata.SetAccessManaPledgeID(created.AccessManaPledgeID())
		outputMetadata.SetConsensusManaPledgeID(created.ConsensusManaPledgeID())
		outputMetadata.SetConfirmationState(confirmation.Confirmed)

		l.Storage.outputStorage.Store(created.Output()).Release()
		l.Storage.outputMetadataStorage.Store(outputMetadata).Release()

		l.Events.OutputCreated.Trigger(created.ID())
	}
}
