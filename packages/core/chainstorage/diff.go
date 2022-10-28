package chainstorage

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

func NewStateDiff() (stateDiff *StateDiff) {
	return &StateDiff{
		CreatedOutputs:         set.NewAdvancedSet[utxo.OutputID](),
		DeletedOutputs:         set.NewAdvancedSet[utxo.OutputID](),
		ConsensusWeightUpdates: make(map[identity.ID]int64),
	}
}

func (d *StateDiff) ApplyCreatedOutputs(createdOutputs []*OutputWithMetadata) (self *StateDiff) {
	for _, createdOutput := range createdOutputs {
		d.ApplyCreatedOutput(createdOutput)
	}

	return d
}

func (d *StateDiff) ApplyCreatedOutput(createdOutput *OutputWithMetadata) {
	d.CreatedOutputs.Add(createdOutput.ID())

	if iotaBalance, exists := createdOutput.IOTABalance(); exists {
		d.ConsensusWeightUpdates[createdOutput.ConsensusManaPledgeID()] += int64(iotaBalance)
	}
}

func (d *StateDiff) ApplyDeletedOutputs(deletedOutputs []*OutputWithMetadata) (self *StateDiff) {
	for _, deletedOutput := range deletedOutputs {
		d.ApplyDeletedOutput(deletedOutput)
	}

	return d
}

func (d *StateDiff) ApplyDeletedOutput(deletedOutput *OutputWithMetadata) {
	if !d.CreatedOutputs.Delete(deletedOutput.ID()) {
		d.DeletedOutputs.Add(deletedOutput.ID())
	}

	if iotaBalance, exists := deletedOutput.IOTABalance(); exists {
		if d.ConsensusWeightUpdates[deletedOutput.ConsensusManaPledgeID()] -= int64(iotaBalance); d.ConsensusWeightUpdates[deletedOutput.ConsensusManaPledgeID()] == 0 {
			delete(d.ConsensusWeightUpdates, deletedOutput.ConsensusManaPledgeID())
		}
	}
}
