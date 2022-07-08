package epochstorage

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin is the plugin instance of the messagelayer plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	epochContentsMutex     sync.RWMutex
	epochContents          = make(map[epoch.Index]*epochContentStorages, 0)
	committableEpochsMutex sync.RWMutex
	committableEpochs      = make([]*epoch.ECRecord, 0)
	epochVotersWeightMutex sync.RWMutex
	epochVotersWeight      = make(map[epoch.Index]map[epoch.ECR]map[identity.ID]float64, 0)
	epochVotersLatestVote  = make(map[identity.ID]*latestVote, 0)

	maxEpochContentsToKeep   = 100
	numEpochContentsToRemove = 20
	minEpochIndex            = epoch.Index(0)

	epochOrderMutex sync.RWMutex
	epochOrderMap   = make(map[epoch.Index]types.Empty, 0)
	epochOrder      = make([]epoch.Index, 0)
)

type epochContentStorages struct {
	spentOutputs   kvstore.KVStore
	createdOutputs kvstore.KVStore
	messageIDs     kvstore.KVStore
	transactionIDs kvstore.KVStore
}

type latestVote struct {
	ei         epoch.Index
	ecr        epoch.ECR
	issuedTime time.Time
}
type dependencies struct {
	dig.In

	Tangle          *tangle.Tangle
	NotarizationMgr *notarization.Manager
}

func init() {
	Plugin = node.NewPlugin("EpochStorage", deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("epochstorage")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	// get the last committed epoch EI as minEpochIndex
	snapshotEC, _ := deps.NotarizationMgr.GetLatestEC()
	minEpochIndex = snapshotEC.EI()

	deps.NotarizationMgr.Events.TangleTreeInserted.Attach(event.NewClosure(func(event *notarization.TangleTreeUpdatedEvent) {
		epochOrderMutex.Lock()
		if _, ok := epochOrderMap[event.EI]; !ok {
			epochOrderMap[event.EI] = types.Void
			epochOrder = append(epochOrder, event.EI)
		}
		epochOrderMutex.Unlock()
		checkEpochContentLimit()

		err := insertMessageToEpoch(event.EI, event.MessageID)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.TangleTreeRemoved.Attach(event.NewClosure(func(event *notarization.TangleTreeUpdatedEvent) {
		err := removeMessageFromEpoch(event.EI, event.MessageID)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.StateMutationTreeInserted.Attach(event.NewClosure(func(event *notarization.StateMutationTreeUpdatedEvent) {
		err := insertTransactionToEpoch(event.EI, event.TransactionID)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.StateMutationTreeRemoved.Attach(event.NewClosure(func(event *notarization.StateMutationTreeUpdatedEvent) {
		err := removeTransactionFromEpoch(event.EI, event.TransactionID)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.UTXOTreeInserted.Attach(event.NewClosure(func(event *notarization.UTXOUpdatedEvent) {
		err := insertOutputsToEpoch(event.EI, event.Spent, event.Created)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.UTXOTreeRemoved.Attach(event.NewClosure(func(event *notarization.UTXOUpdatedEvent) {
		err := removeOutputsFromEpoch(event.EI, event.Spent, event.Created)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		committableEpochsMutex.Lock()
		defer committableEpochsMutex.Unlock()
		committableEpochs = append(committableEpochs, event.ECRecord)
	}))

	deps.Tangle.ConfirmationOracle.Events().MessageAccepted.Attach(event.NewClosure(func(event *tangle.MessageAcceptedEvent) {
		message := event.Message
		saveEpochVotersWeight(message)
	}))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("EpochStorage", func(ctx context.Context) {
		<-ctx.Done()
	}, shutdown.PriorityNotarization); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func checkEpochContentLimit() {
	epochOrderMutex.Lock()
	if len(epochOrder) <= maxEpochContentsToKeep {
		epochOrderMutex.Unlock()
		return
	}

	// sort the order list to remove the oldest ones.
	sort.Slice(epochOrder, func(i, j int) bool {
		return epochOrder[i] < epochOrder[j]
	})

	var epochToRemove []epoch.Index
	copy(epochToRemove, epochOrder[:numEpochContentsToRemove])
	// remove the first numEpochContentsToRemove epochs
	epochOrder = epochOrder[numEpochContentsToRemove:]

	for _, i := range epochToRemove {
		delete(epochOrderMap, i)
	}
	epochOrderMutex.Unlock()

	epochContentsMutex.Lock()
	for _, i := range epochToRemove {
		delete(epochContents, i)
	}
	epochContentsMutex.Unlock()

	epochVotersWeightMutex.Lock()
	for _, i := range epochToRemove {
		delete(epochVotersWeight, i)
	}
	epochVotersWeightMutex.Unlock()

	// update minEpochIndex
	minEpochIndex = epochOrder[0]

	committableEpochsMutex.Lock()
	if len(committableEpochs) < maxEpochContentsToKeep {
		committableEpochsMutex.Unlock()
		return
	}
	committableEpochs = committableEpochs[len(committableEpochs)-maxEpochContentsToKeep:]
	committableEpochsMutex.Unlock()
}

func GetCommittableEpochs() (ecRecords map[epoch.Index]*epoch.ECRecord) {
	ecRecords = make(map[epoch.Index]*epoch.ECRecord, 0)

	committableEpochsMutex.RLock()
	for _, record := range committableEpochs {
		ecRecords[record.EI()] = record
	}
	committableEpochsMutex.RUnlock()

	return
}

func GetPendingBranchCount() map[epoch.Index]uint64 {
	return deps.NotarizationMgr.PendingConflictsCountAll()
}

func GetEpochMessages(ei epoch.Index) ([]tangle.MessageID, error) {
	stores, err := getEpochContentStorage(ei)
	if err != nil {
		return []tangle.MessageID{}, err
	}

	var msgIDs []tangle.MessageID
	stores.messageIDs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var msgID tangle.MessageID
		if _, err := msgID.Decode(key); err != nil {
			panic("MessageID could not be parsed!")
		}
		msgIDs = append(msgIDs, msgID)
		return true
	})

	return msgIDs, nil
}

func GetEpochTransactions(ei epoch.Index) ([]utxo.TransactionID, error) {
	stores, err := getEpochContentStorage(ei)
	if err != nil {
		return []utxo.TransactionID{}, err
	}

	var txIDs []utxo.TransactionID
	stores.transactionIDs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var txID utxo.TransactionID
		if _, err := txID.Decode(key); err != nil {
			panic("TransactionID could not be parsed!")
		}
		txIDs = append(txIDs, txID)
		return true
	})

	return txIDs, nil
}

func GetEpochUTXOs(ei epoch.Index) (spent, created []utxo.OutputID, err error) {
	stores, err := getEpochContentStorage(ei)
	if err != nil {
		return []utxo.OutputID{}, []utxo.OutputID{}, err
	}

	stores.createdOutputs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var outputID utxo.OutputID
		if err = outputID.FromBytes(key); err != nil {
			panic(err)
		}
		created = append(created, outputID)

		return true
	})

	stores.spentOutputs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var outputID utxo.OutputID
		if err = outputID.FromBytes(key); err != nil {
			panic(err)
		}
		spent = append(spent, outputID)

		return true
	})

	return spent, created, nil
}

func GetEpochVotersWeight(ei epoch.Index) (weights map[epoch.ECR]map[identity.ID]float64) {
	epochVotersWeightMutex.RLock()
	defer epochVotersWeightMutex.RUnlock()
	if _, ok := epochVotersWeight[ei]; !ok {
		return
	}

	weights = make(map[epoch.ECR]map[identity.ID]float64, len(epochVotersWeight[ei]))
	for ecr, voterWeights := range epochVotersWeight[ei] {
		subDuplicate := make(map[identity.ID]float64, len(voterWeights))
		for id, w := range voterWeights {
			subDuplicate[id] = w
		}
		weights[ecr] = subDuplicate
	}
	return weights
}

func getEpochContentStorage(ei epoch.Index) (*epochContentStorages, error) {
	if ei < minEpochIndex {
		return nil, errors.New("Epoch storage no longer exists")
	}

	epochContentsMutex.RLock()
	stores, ok := epochContents[ei]
	epochContentsMutex.RUnlock()

	if !ok {
		stores = newEpochContentStorage()
		epochContentsMutex.Lock()
		epochContents[ei] = stores
		epochContentsMutex.Unlock()
	}

	return stores, nil
}

func newEpochContentStorage() *epochContentStorages {
	db, _ := database.NewMemDB()

	// keep data in temporary storage
	return &epochContentStorages{
		spentOutputs:   db.NewStore(),
		createdOutputs: db.NewStore(),
		transactionIDs: db.NewStore(),
		messageIDs:     db.NewStore(),
	}
}

func insertMessageToEpoch(ei epoch.Index, msgID tangle.MessageID) error {
	epochContentStorage, err := getEpochContentStorage(ei)
	if err != nil {
		return err
	}

	if err := epochContentStorage.messageIDs.Set(msgID.Bytes(), msgID.Bytes()); err != nil {
		return errors.New("Fail to insert Message to epoch store")
	}
	return nil
}

func removeMessageFromEpoch(ei epoch.Index, msgID tangle.MessageID) error {
	epochContentStorage, err := getEpochContentStorage(ei)
	if err != nil {
		return err
	}

	if err := epochContentStorage.messageIDs.Delete(msgID.Bytes()); err != nil {
		return errors.New("Fail to remove Message from epoch store")
	}
	return nil
}

func insertTransactionToEpoch(ei epoch.Index, txID utxo.TransactionID) error {
	epochContentStorage, err := getEpochContentStorage(ei)
	if err != nil {
		return err
	}

	if err := epochContentStorage.transactionIDs.Set(txID.Bytes(), txID.Bytes()); err != nil {
		return errors.New("Fail to insert Transaction to epoch store")
	}
	return nil
}

func removeTransactionFromEpoch(ei epoch.Index, txID utxo.TransactionID) error {
	epochContentStorage, err := getEpochContentStorage(ei)
	if err != nil {
		return err
	}

	if err := epochContentStorage.transactionIDs.Delete(txID.Bytes()); err != nil {
		return errors.New("Fail to remove Transaction from epoch store")
	}
	return nil
}

func insertOutputsToEpoch(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) error {
	epochContentStorage, err := getEpochContentStorage(ei)
	if err != nil {
		return err
	}

	for _, s := range spent {
		if err := epochContentStorage.spentOutputs.Set(s.ID().Bytes(), s.ID().Bytes()); err != nil {
			return errors.New("Fail to insert spent output to epoch store")
		}
	}

	for _, c := range created {
		if err := epochContentStorage.createdOutputs.Set(c.ID().Bytes(), c.ID().Bytes()); err != nil {
			return errors.New("Fail to insert created output to epoch store")
		}
	}

	return nil
}

func removeOutputsFromEpoch(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) error {
	epochContentStorage, err := getEpochContentStorage(ei)
	if err != nil {
		return err
	}

	for _, s := range spent {
		if err := epochContentStorage.spentOutputs.Delete(s.ID().Bytes()); err != nil {
			return errors.New("Fail to remove spent output from epoch store")
		}
	}

	for _, c := range created {
		if err := epochContentStorage.createdOutputs.Delete(c.ID().Bytes()); err != nil {
			return errors.New("Fail to remove created output from epoch store")
		}
	}

	return nil
}

func saveEpochVotersWeight(message *tangle.Message) {
	voter := identity.NewID(message.IssuerPublicKey())
	activeWeights, _ := deps.Tangle.WeightProvider.WeightsOfRelevantVoters()

	epochVotersWeightMutex.Lock()
	defer epochVotersWeightMutex.Unlock()
	epochIndex := message.M.EI
	ecr := message.M.ECR
	if _, ok := epochVotersWeight[epochIndex]; !ok {
		epochVotersWeight[epochIndex] = make(map[epoch.ECR]map[identity.ID]float64)
	}
	if _, ok := epochVotersWeight[epochIndex][ecr]; !ok {
		epochVotersWeight[epochIndex][ecr] = make(map[identity.ID]float64)
	}

	vote, ok := epochVotersLatestVote[voter]
	if ok {
		if vote.ei == epochIndex && vote.ecr != ecr && vote.issuedTime.Before(message.M.IssuingTime) {
			delete(epochVotersWeight[vote.ei][vote.ecr], voter)
		}
	}
	epochVotersLatestVote[voter] = &latestVote{ei: epochIndex, ecr: ecr, issuedTime: message.M.IssuingTime}
	epochVotersWeight[epochIndex][ecr][voter] = activeWeights[voter]
}
