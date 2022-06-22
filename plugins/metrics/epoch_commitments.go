package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// EpochContent is a storable object represents the epoch and its content like UTXO ids, message ids, or transaction ids.
type EpochContent struct {
	model.Storable[epoch.Index, EpochContent, *EpochContent, epochContent] `serix:"0"`
}

type epochContent struct {
	EI      epoch.Index `serix:"0"`
	Content []string    `serix:"1"`
}

// NewEpochContent creates and returns an EpochContent of the given EI.
func NewEpochContent(ei epoch.Index) (new *EpochContent) {
	new = model.NewStorable[epoch.Index, EpochContent](&epochContent{
		EI: ei,
	})
	new.SetID(ei)
	return
}

const (
	storagePrefixUTXOs byte = iota
	storagePrefixMessages
	storagePrefixTransactions
)

type EpochCommitmentsMetrics struct {
	epochCommitmentsMutex sync.RWMutex
	epochCommitments      map[epoch.Index]*epoch.ECRecord

	epochVotersWeightMutex sync.RWMutex
	epochVotersWeight      map[epoch.Index]map[identity.ID]float64

	epochUTXOsMutex        sync.RWMutex
	epochUTXOs             *objectstorage.ObjectStorage[*EpochContent]
	epochMessagesMutex     sync.RWMutex
	epochMessages          *objectstorage.ObjectStorage[*EpochContent]
	epochTransactionsMutex sync.RWMutex
	epochTransactions      *objectstorage.ObjectStorage[*EpochContent]
}

func NewEpochCommitmentsMetrics(storage kvstore.KVStore) (*EpochCommitmentsMetrics, error) {
	metrics := &EpochCommitmentsMetrics{
		epochCommitments:  map[epoch.Index]*epoch.ECRecord{},
		epochVotersWeight: map[epoch.Index]map[identity.ID]float64{},
		epochUTXOs: objectstorage.NewStructStorage[EpochContent](
			objectstorage.NewStoreWithRealm(storage, database.PrefixNotarization, storagePrefixUTXOs),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		epochMessages: objectstorage.NewStructStorage[EpochContent](
			objectstorage.NewStoreWithRealm(storage, database.PrefixNotarization, storagePrefixMessages),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		epochTransactions: objectstorage.NewStructStorage[EpochContent](
			objectstorage.NewStoreWithRealm(storage, database.PrefixNotarization, storagePrefixTransactions),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
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

func (m *EpochCommitmentsMetrics) saveEpochUTXOs(tx *devnetvm.Transaction) {
	earliestAttachment := deps.Tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
	ei := deps.NotarizationMgr.EpochManager().TimeToEI(earliestAttachment.IssuingTime())
	createdOutputs := tx.Essence().Outputs().UTXOOutputs()
	m.epochUTXOsMutex.Lock()
	defer m.epochUTXOsMutex.Unlock()
	obj := m.epochUTXOs.Load(ei.Bytes())
	t := obj.Get()
	if t == nil {
		t = NewEpochContent(ei)
	}
	for _, o := range createdOutputs {
		t.M.Content = append(t.M.Content, o.ID().String())
	}
	m.epochUTXOs.Store(t)
}

func (m *EpochCommitmentsMetrics) GetEpochUTXOs() map[epoch.Index][]string {
	m.epochUTXOsMutex.RLock()
	defer m.epochUTXOsMutex.RUnlock()
	duplicate := make(map[epoch.Index][]string, 0)
	m.epochUTXOs.ForEach(func(_ []byte, obj *objectstorage.CachedObject[*EpochContent]) bool {
		ec := obj.Get()
		duplicate[ec.M.EI] = ec.M.Content
		return true
	})
	return duplicate
}

func (m *EpochCommitmentsMetrics) saveEpochMessage(msg *tangle.Message) {
	ei := deps.NotarizationMgr.EpochManager().TimeToEI(msg.IssuingTime())
	m.epochMessagesMutex.Lock()
	defer m.epochMessagesMutex.Unlock()
	obj := m.epochMessages.Load(ei.Bytes())
	t := obj.Get()
	if t == nil {
		t = NewEpochContent(ei)
	}
	t.M.Content = append(t.M.Content, msg.ID().String())
	m.epochMessages.Store(t)
}

func (m *EpochCommitmentsMetrics) GetEpochMessages() map[epoch.Index][]string {
	m.epochMessagesMutex.RLock()
	defer m.epochMessagesMutex.RUnlock()
	duplicate := make(map[epoch.Index][]string, 0)
	m.epochMessages.ForEach(func(_ []byte, obj *objectstorage.CachedObject[*EpochContent]) bool {
		ec := obj.Get()
		duplicate[ec.M.EI] = ec.M.Content
		return true
	})
	return duplicate
}

func (m *EpochCommitmentsMetrics) saveEpochTransaction(tx utxo.Transaction) {
	earliestAttachment := deps.Tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
	ei := deps.NotarizationMgr.EpochManager().TimeToEI(earliestAttachment.IssuingTime())
	m.epochTransactionsMutex.Lock()
	defer m.epochTransactionsMutex.Unlock()
	obj := m.epochTransactions.Load(ei.Bytes())
	t := obj.Get()
	if t == nil {
		t = NewEpochContent(ei)
	}
	t.M.Content = append(t.M.Content, tx.ID().String())
	m.epochTransactions.Store(t)
}

func (m *EpochCommitmentsMetrics) GetEpochTransactions() map[epoch.Index][]string {
	m.epochTransactionsMutex.RLock()
	defer m.epochTransactionsMutex.RUnlock()
	duplicate := make(map[epoch.Index][]string, 0)
	m.epochTransactions.ForEach(func(_ []byte, obj *objectstorage.CachedObject[*EpochContent]) bool {
		ec := obj.Get()
		duplicate[ec.M.EI] = ec.M.Content
		return true
	})
	return duplicate
}

func GetLastCommittedEpoch() epoch.ECRecord {
	epochRecord, err := deps.NotarizationMgr.LastCommittedEpoch()
	if err != nil {
		Plugin.LogError("Notarization manager failed to return last committed epoch", err)
	}
	return *epochRecord
}

func GetPendingBranchCount() map[epoch.Index]uint64 {
	return deps.NotarizationMgr.PendingConflictsCountAll()
}
