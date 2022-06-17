package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type EpochInfo struct {
	EI     epoch.Index
	ECR    string
	PrevEC string
}

var (
	epochCommitmentsMutex = sync.RWMutex{}
	epochCommitments      map[epoch.Index]EpochInfo

	epochVotersWeightMutex sync.RWMutex
	epochVotersWeight      = map[epoch.Index]map[identity.ID]float64{}

	epochUTXOsMutex sync.RWMutex
	epochUTXOs      = map[epoch.Index][]string{}

	epochMessagesMutex sync.RWMutex
	epochMessages      = map[epoch.Index][]string{}

	epochTransactionsMutex sync.RWMutex
	epochTransactions      = map[epoch.Index][]string{}
)

func saveCommittedEpoch(ecr *epoch.ECRecord) {
	epochCommitmentsMutex.Lock()
	defer epochCommitmentsMutex.Unlock()
	epochCommitments[ecr.EI()] = EpochInfo{
		EI:     ecr.EI(),
		ECR:    ecr.M.ECR.String(),
		PrevEC: ecr.M.PrevEC.String(),
	}
}

func GetCommittedEpochs() map[epoch.Index]EpochInfo {
	epochCommitmentsMutex.RLock()
	defer epochCommitmentsMutex.RUnlock()
	duplicate := make(map[epoch.Index]EpochInfo, len(epochCommitments))
	for k, v := range epochCommitments {
		duplicate[k] = v
	}
	return duplicate
}

func saveEpochVotersWeight(message *tangle.Message) {
	branchesOfMessage, err := deps.Tangle.Booker.MessageBranchIDs(message.ID())
	if err != nil {
		Plugin.LogError(err)
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
}

func GetEpochVotersWeight() map[epoch.Index]map[identity.ID]float64 {
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

func saveEpochUTXOs(tx *devnetvm.Transaction) {
	earliestAttachment := deps.Tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
	ei := deps.NotarizationMgr.EpochManager().TimeToEI(earliestAttachment.IssuingTime())
	createdOutputs := tx.Essence().Outputs().UTXOOutputs()
	epochUTXOsMutex.Lock()
	defer epochUTXOsMutex.Unlock()
	for _, o := range createdOutputs {
		epochUTXOs[ei] = append(epochUTXOs[ei], o.ID().String())
	}
}

func GetEpochUTXOs() map[epoch.Index][]string {
	epochUTXOsMutex.RLock()
	defer epochUTXOsMutex.RUnlock()
	duplicate := make(map[epoch.Index][]string, len(epochUTXOs))
	for k, v := range epochUTXOs {
		duplicate[k] = v
	}
	return duplicate
}

func saveEpochMessage(msg *tangle.Message) {
	ei := deps.NotarizationMgr.EpochManager().TimeToEI(msg.IssuingTime())
	epochMessagesMutex.Lock()
	defer epochMessagesMutex.Unlock()
	epochMessages[ei] = append(epochMessages[ei], msg.ID().String())
}

func GetEpochMessages() map[epoch.Index][]string {
	epochMessagesMutex.RLock()
	defer epochMessagesMutex.RUnlock()
	duplicate := make(map[epoch.Index][]string, len(epochMessages))
	for k, v := range epochMessages {
		duplicate[k] = v
	}
	return duplicate
}

func saveEpochTransaction(tx utxo.Transaction) {
	earliestAttachment := deps.Tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
	ei := deps.NotarizationMgr.EpochManager().TimeToEI(earliestAttachment.IssuingTime())
	epochTransactionsMutex.Lock()
	defer epochTransactionsMutex.Unlock()
	epochTransactions[ei] = append(epochTransactions[ei], tx.ID().String())
}

func GetEpochTransactions() map[epoch.Index][]string {
	epochTransactionsMutex.RLock()
	defer epochTransactionsMutex.RUnlock()
	duplicate := make(map[epoch.Index][]string, len(epochTransactions))
	for k, v := range epochTransactions {
		duplicate[k] = v
	}
	return duplicate
}

func GetLastCommittedEpoch() EpochInfo {
	epochRecord, err := deps.NotarizationMgr.LastCommittedEpoch()
	if err != nil {
		Plugin.LogError("Notarization manager failed to return last committed epoch", err)
	}
	return EpochInfo{
		EI:     epochRecord.EI(),
		ECR:    epochRecord.ECR().String(),
		PrevEC: epochRecord.PrevEC().String(),
	}
}

func GetPendingBranchCount() map[epoch.Index]uint64 {
	return deps.NotarizationMgr.PendingConflictsCountAll()
}
