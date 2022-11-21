package models

import (
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type StateDiff struct {
	CreatedOutputs *set.AdvancedSet[utxo.OutputID]
	DeletedOutputs *set.AdvancedSet[utxo.OutputID]
}

func NewMemoryStateDiff() (stateDiff *StateDiff) {
	return &StateDiff{
		CreatedOutputs: set.NewAdvancedSet[utxo.OutputID](),
		DeletedOutputs: set.NewAdvancedSet[utxo.OutputID](),
	}
}

func (m *StateDiff) ApplyCreatedOutputs(createdOutputs []*OutputWithMetadata) (self *StateDiff) {
	for _, createdOutput := range createdOutputs {
		m.ApplyCreatedOutput(createdOutput)
	}

	return m
}

func (m *StateDiff) ApplyCreatedOutput(createdOutput *OutputWithMetadata) {
	m.CreatedOutputs.Add(createdOutput.ID())
}

func (m *StateDiff) ApplyDeletedOutputs(deletedOutputs []*OutputWithMetadata) (self *StateDiff) {
	for _, deletedOutput := range deletedOutputs {
		m.ApplyDeletedOutput(deletedOutput)
	}

	return m
}

func (m *StateDiff) ApplyDeletedOutput(deletedOutput *OutputWithMetadata) {
	if !m.CreatedOutputs.Delete(deletedOutput.ID()) {
		m.DeletedOutputs.Add(deletedOutput.ID())
	}
}
