package ledger

import (
	"context"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type Ledger struct {
	Events *Events

	unspentOutputs   kvstore.KVStore
	unspentOutputIDs *ads.Set[utxo.OutputID]
	consensusWeights *ads.Map[identity.ID, TimedBalance, *TimedBalance]

	LedgerStateDiffs *StateDiffs
}

func New(unspentOutputsStorage, unspentOutputIDsStore, consensusWeightStore kvstore.KVStore, ledgerStateDiffStoreProvider func(epoch.Index) kvstore.KVStore) (newState *Ledger) {
	return &Ledger{
		Events:           NewEvents(),
		unspentOutputs:   unspentOutputsStorage,
		unspentOutputIDs: ads.NewSet[utxo.OutputID](unspentOutputIDsStore),
		consensusWeights: ads.NewMap[identity.ID, TimedBalance](consensusWeightStore),
		LedgerStateDiffs: &StateDiffs{BucketedStorage: ledgerStateDiffStoreProvider},
	}
}

func (s *Ledger) UnspentOutputsStorage() (unspentOutputsStorage kvstore.KVStore) {
	return s.unspentOutputs
}

func (s *Ledger) ApplyEpoch(index epoch.Index) (stateRoot, manaRoot types.Identifier) {
	return s.applyStateDiff(index, s.LedgerStateDiffs.StateDiff(index), s.unspentOutputIDs.Add, void(s.unspentOutputIDs.Delete))
}

func (s *Ledger) RollbackEpochStateDiff(index epoch.Index, stateDiff *MemoryStateDiff) (stateRoot, manaRoot types.Identifier) {
	return s.applyStateDiff(index, stateDiff, void(s.unspentOutputIDs.Delete), s.unspentOutputIDs.Add)
}

func (s *Ledger) ImportUnspentOutputIDs(outputIDs []utxo.OutputID) {
	for _, outputID := range outputIDs {
		s.unspentOutputIDs.Add(outputID)
	}
}

func (s *Ledger) applyStateDiff(index epoch.Index, stateDiff *MemoryStateDiff, create, delete func(id utxo.OutputID)) (stateRoot, manaRoot types.Identifier) {
	for it := stateDiff.CreatedOutputs.Iterator(); it.HasNext(); {
		create(it.Next())
	}
	for it := stateDiff.DeletedOutputs.Iterator(); it.HasNext(); {
		delete(it.Next())
	}

	consensusWeightUpdates := make(map[identity.ID]*TimedBalance)

	for id, diff := range stateDiff.ConsensusWeightUpdates {
		if diff == 0 {
			continue
		}

		timedBalance := lo.Return1(s.consensusWeights.Get(id))
		if index == timedBalance.LastUpdated {
			continue
		}

		timedBalance.Balance += diff * int64(lo.Compare(index, timedBalance.LastUpdated))
		timedBalance.LastUpdated = index

		consensusWeightUpdates[id] = timedBalance

		if timedBalance.Balance == 0 {
			s.consensusWeights.Delete(id)
		} else {
			s.consensusWeights.Set(id, timedBalance)
		}
	}

	s.Events.ConsensusWeightsUpdated.Trigger(consensusWeightUpdates)

	return s.unspentOutputIDs.Root(), s.consensusWeights.Root()
}

type TimedBalance struct {
	Balance     int64       `serix:"0"`
	LastUpdated epoch.Index `serix:"1"`
}

func NewTimedBalance(balance int64, lastUpdated epoch.Index) (timedBalance *TimedBalance) {
	return &TimedBalance{
		Balance:     balance,
		LastUpdated: lastUpdated,
	}
}

func (t TimedBalance) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), t)
}

func (t *TimedBalance) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, t)
}

func void[A, B any](f func(A) B) func(A) {
	return func(a A) { f(a) }
}
