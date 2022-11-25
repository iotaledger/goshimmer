package ledgerstate

import (
	"context"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage/models"
	"github.com/iotaledger/hive.go/core/kvstore"
)

type UnspentOutputIDs struct {
	lastCommittedEpoch      epoch.Index
	lastCommittedEpochMutex sync.RWMutex
	batchedEpochIndex       epoch.Index
	batchedEpochMutex       sync.RWMutex
	batchedCreatedOutputIDs utxo.OutputIDs
	batchedSpentOutputIDs   utxo.OutputIDs

	*ads.Set[utxo.OutputID]
}

func NewUnspentOutputIDs(store kvstore.KVStore) *UnspentOutputIDs {
	return &UnspentOutputIDs{
		Set: ads.NewSet[utxo.OutputID](store),
	}
}

func (u *UnspentOutputIDs) LastCommittedEpoch() epoch.Index {
	u.lastCommittedEpochMutex.RLock()
	defer u.lastCommittedEpochMutex.RUnlock()

	return u.lastCommittedEpoch
}

func (u *UnspentOutputIDs) SetLastCommittedEpoch(index epoch.Index) {
	u.lastCommittedEpochMutex.Lock()
	defer u.lastCommittedEpochMutex.Unlock()

	u.lastCommittedEpoch = index
}

func (u *UnspentOutputIDs) Begin(committedEpoch epoch.Index) {
	u.setBatchedEpoch(committedEpoch)
	u.batchedCreatedOutputIDs = utxo.NewOutputIDs()
	u.batchedSpentOutputIDs = utxo.NewOutputIDs()
}

func (u *UnspentOutputIDs) ImportOutputs(outputs []*models.OutputWithMetadata) {
	for _, output := range outputs {
		u.ProcessCreatedOutput(output)
	}
}

func (u *UnspentOutputIDs) ProcessCreatedOutput(output *models.OutputWithMetadata) {
	if u.batchedEpoch() == 0 {
		u.Add(output.Output().ID())
	} else if !u.batchedSpentOutputIDs.Delete(output.Output().ID()) {
		u.batchedCreatedOutputIDs.Add(output.Output().ID())
	}
}

func (u *UnspentOutputIDs) ProcessSpentOutput(output *models.OutputWithMetadata) {
	if u.batchedEpoch() == 0 {
		u.Delete(output.Output().ID())
	} else if !u.batchedCreatedOutputIDs.Delete(output.Output().ID()) {
		u.batchedSpentOutputIDs.Add(output.Output().ID())
	}
}

func (u *UnspentOutputIDs) Commit() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())
	go func() {
		for it := u.batchedCreatedOutputIDs.Iterator(); it.HasNext(); {
			u.Add(it.Next())
		}
		for it := u.batchedSpentOutputIDs.Iterator(); it.HasNext(); {
			u.Delete(it.Next())
		}

		u.SetLastCommittedEpoch(u.batchedEpoch())
		u.setBatchedEpoch(0)

		done()
	}()

	return ctx
}

func (u *UnspentOutputIDs) batchedEpoch() epoch.Index {
	u.batchedEpochMutex.RLock()
	defer u.batchedEpochMutex.RUnlock()

	return u.batchedEpochIndex
}

func (u *UnspentOutputIDs) setBatchedEpoch(index epoch.Index) {
	u.batchedEpochMutex.Lock()
	defer u.batchedEpochMutex.Unlock()

	if index != 0 && u.batchedEpochIndex != 0 {
		panic("a batch is already in progress")
	}

	u.batchedEpochIndex = index
}

var _ DiffConsumer = &UnspentOutputIDs{}
