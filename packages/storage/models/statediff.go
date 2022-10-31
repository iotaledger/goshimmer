package models

import (
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type StateDiff struct {
	CreatedOutputs         *set.AdvancedSet[utxo.OutputID]
	DeletedOutputs         *set.AdvancedSet[utxo.OutputID]
	ConsensusWeightUpdates map[identity.ID]int64
}

func NewMemoryStateDiff() (stateDiff *StateDiff) {
	return &StateDiff{
		CreatedOutputs:         set.NewAdvancedSet[utxo.OutputID](),
		DeletedOutputs:         set.NewAdvancedSet[utxo.OutputID](),
		ConsensusWeightUpdates: make(map[identity.ID]int64),
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

	if iotaBalance, exists := createdOutput.IOTABalance(); exists {
		m.ConsensusWeightUpdates[createdOutput.ConsensusManaPledgeID()] += int64(iotaBalance)
	}
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

	if iotaBalance, exists := deletedOutput.IOTABalance(); exists {
		if m.ConsensusWeightUpdates[deletedOutput.ConsensusManaPledgeID()] -= int64(iotaBalance); m.ConsensusWeightUpdates[deletedOutput.ConsensusManaPledgeID()] == 0 {
			delete(m.ConsensusWeightUpdates, deletedOutput.ConsensusManaPledgeID())
		}
	}
}
