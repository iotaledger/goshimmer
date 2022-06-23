package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type EpochCommitmentsMetrics struct {
	epochCommitmentsMutex sync.RWMutex
	epochCommitments      map[epoch.Index]*epoch.ECRecord

	epochVotersWeightMutex sync.RWMutex
	epochVotersWeight      map[epoch.Index]map[identity.ID]float64
}

func NewEpochCommitmentsMetrics() (*EpochCommitmentsMetrics, error) {
	metrics := &EpochCommitmentsMetrics{
		epochCommitments:  map[epoch.Index]*epoch.ECRecord{},
		epochVotersWeight: map[epoch.Index]map[identity.ID]float64{},
	}
	return metrics, nil
}

func (m *EpochCommitmentsMetrics) saveCommittedEpoch(ecr *epoch.ECRecord) {
	m.epochCommitmentsMutex.Lock()
	defer m.epochCommitmentsMutex.Unlock()
	m.epochCommitments[ecr.EI()] = ecr
}

func (m *EpochCommitmentsMetrics) GetCommittedEpochs() map[epoch.Index]*epoch.ECRecord {
	m.epochCommitmentsMutex.RLock()
	defer m.epochCommitmentsMutex.RUnlock()
	duplicate := make(map[epoch.Index]*epoch.ECRecord, len(m.epochCommitments))
	for k, v := range m.epochCommitments {
		duplicate[k] = v
	}
	return duplicate
}

func (m *EpochCommitmentsMetrics) saveEpochVotersWeight(message *tangle.Message) {
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
	m.epochVotersWeightMutex.Lock()
	defer m.epochVotersWeightMutex.Unlock()

	epochIndex := message.M.EI
	if _, ok := m.epochVotersWeight[epochIndex]; !ok {
		m.epochVotersWeight[epochIndex] = map[identity.ID]float64{}
	}
	m.epochVotersWeight[epochIndex][voter] += totalWeight
}

func (m *EpochCommitmentsMetrics) GetEpochVotersWeight() map[epoch.Index]map[identity.ID]float64 {
	m.epochVotersWeightMutex.RLock()
	defer m.epochVotersWeightMutex.RUnlock()
	duplicate := make(map[epoch.Index]map[identity.ID]float64, len(m.epochVotersWeight))
	for k, v := range m.epochVotersWeight {
		subDuplicate := make(map[identity.ID]float64, len(v))
		for subK, subV := range v {
			subDuplicate[subK] = subV
		}
		duplicate[k] = subDuplicate
	}
	return duplicate
}

func GetEpochUTXOs(ei epoch.Index) (spent, created []utxo.OutputID) {
	return deps.NotarizationMgr.GetEpochUTXOs(ei)
}

func GetEpochMessages(ei epoch.Index) ([]tangle.MessageID, error) {
	return deps.NotarizationMgr.GetEpochMessages(ei)
}

func GetEpochTransactions(ei epoch.Index) ([]utxo.TransactionID, error) {
	return deps.NotarizationMgr.GetEpochTransactions(ei)
}

func GetLastCommittedEpoch() *epoch.ECRecord {
	epochRecord, err := deps.NotarizationMgr.LastCommittedEpoch()
	if err != nil {
		Plugin.LogError("Notarization manager failed to return last committed epoch", err)
	}
	return epochRecord
}

func GetPendingBranchCount() map[epoch.Index]uint64 {
	return deps.NotarizationMgr.PendingConflictsCountAll()
}
