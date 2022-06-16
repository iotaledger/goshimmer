package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	epochCommitmentsMutex = sync.RWMutex{}
	epochCommitments      map[epoch.Index]*epoch.ECRecord

	epochVotersWeightMutex sync.RWMutex
	epochVotersWeight      = map[epoch.Index]map[identity.ID]float64{}
)

func saveCommittedEpoch(ecr *epoch.ECRecord) {
	epochCommitmentsMutex.Lock()
	defer epochCommitmentsMutex.Unlock()
	epochCommitments[ecr.EI()] = ecr
}

func getCommittedEpochs() map[epoch.Index]*epoch.ECRecord {
	epochCommitmentsMutex.RLock()
	defer epochCommitmentsMutex.RUnlock()
	duplicate := make(map[epoch.Index]*epoch.ECRecord, len(epochCommitments))
	for k, v := range epochCommitments {
		duplicate[k] = v
	}
	return duplicate
}

func saveEpochVotersWeight(message *tangle.Message) error {
	branchesOfMessage, err := deps.Tangle.Booker.MessageBranchIDs(message.ID())
	if err != nil {
		Plugin.Panic(err)
	}
	voter := identity.NewID(message.IssuerPublicKey())
	var totalWeight float64
	_ = branchesOfMessage.ForEach(func(txID utxo.TransactionID) error {
		weight := deps.Tangle.ApprovalWeightManager.WeightOfBranch(txID)
		totalWeight += weight
		return nil
	})
	epochVotersWeightMutex.Lock()
	defer epochVotersWeightMutex.Unlock()

	epochIndex := message.M.EI
	if _, ok := epochVotersWeight[epochIndex]; !ok {
		epochVotersWeight[epochIndex] = map[identity.ID]float64{}
	}
	epochVotersWeight[epochIndex][voter] += totalWeight
	return nil
}

func getEpochVotersWeight() map[epoch.Index]map[identity.ID]float64 {
	epochVotersWeightMutex.RLock()
	defer epochVotersWeightMutex.RUnlock()
	duplicate := make(map[epoch.Index]map[identity.ID]float64, len(epochVotersWeight))
	for k, v := range epochVotersWeight {
		subDuplicate := make(map[identity.ID]float64, len(v))
		for subK, subV := range v {
			subDuplicate[subK] = subV
		}
		duplicate[k] = subDuplicate
	}
	return duplicate

}
