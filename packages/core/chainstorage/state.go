package chainstorage

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type State struct {
	unspentOutputIDs *ads.Set[utxo.OutputID]
	consensusWeights *ads.Map[identity.ID, storable.SerializableInt64, *storable.SerializableInt64]
}

func New(unspentOutputIDsStore, consensusWeightsStore kvstore.KVStore) (newState *State) {
	return &State{
		unspentOutputIDs: ads.NewSet[utxo.OutputID](unspentOutputIDsStore),
		consensusWeights: ads.NewMap[identity.ID, storable.SerializableInt64](consensusWeightsStore),
	}
}

func (s *State) Import(index epoch.Index, chunk []*OutputWithMetadata) {
	s.Apply(index, NewStateDiff().ApplyCreatedOutputs(chunk))
}

func (s *State) Apply(index epoch.Index, stateDiff *StateDiff) (stateRoot, manaRoot types.Identifier) {
	return s.apply(stateDiff, 1)
}

func (s *State) Rollback(stateDiff *StateDiff) (stateRoot, manaRoot types.Identifier) {
	return s.apply(stateDiff, -1)
}

func (s *State) apply(stateDiff *StateDiff, direction int64) (stateRoot, manaRoot types.Identifier) {
	for it := stateDiff.CreatedOutputs.Iterator(); it.HasNext(); {
		if direction == 1 {
			s.unspentOutputIDs.Add(it.Next())
		} else {
			s.unspentOutputIDs.Delete(it.Next())
		}
	}

	for it := stateDiff.DeletedOutputs.Iterator(); it.HasNext(); {
		if direction == 1 {
			s.unspentOutputIDs.Delete(it.Next())
		} else {
			s.unspentOutputIDs.Add(it.Next())
		}
	}

	for id, diff := range stateDiff.ConsensusWeightUpdates {
		if diff != 0 {
			consensusWeightUpdate := &ConsensusWeightUpdate{
				OldAmount: int64(lo.Return1(s.consensusWeights.Get(id))),
				Diff:      direction * diff,
			}

			if newAmount := consensusWeightUpdate.OldAmount + consensusWeightUpdate.Diff; newAmount == 0 {
				s.consensusWeights.Delete(id)
			} else {
				s.consensusWeights.Set(id, storable.SerializableInt64(newAmount))
			}
		}
	}

	// todo: Trigger consensus weight change event

	return s.unspentOutputIDs.Root(), s.consensusWeights.Root()
}

type ConsensusWeightUpdate struct {
	OldAmount int64
	Diff      int64
}

func newConsensusWeightUpdate(oldAmount int64) *ConsensusWeightUpdate {
	return &ConsensusWeightUpdate{
		OldAmount: oldAmount,
	}
}

// ConsensusWeightsUpdatedEvent is a container that acts as a dictionary for the EpochCommittable event related parameters.
type ConsensusWeightsUpdatedEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index

	AmountAndDiffByIdentity map[identity.ID]*struct{}
}
