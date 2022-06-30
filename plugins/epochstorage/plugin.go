package epochstorage

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/serix"

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
	Plugin             *node.Plugin
	lastCommittedEpoch = epoch.Index(0)
	deps               = new(dependencies)
	epochContents      = make(map[epoch.Index]*epochContentStorages, 0)
	committedEpochs    = make(map[epoch.Index]*epoch.ECRecord)
)

type epochContentStorages struct {
	spentOutputs   kvstore.KVStore
	createdOutputs kvstore.KVStore
	messageIDs     kvstore.KVStore
	transactionIDs kvstore.KVStore
}

type dependencies struct {
	dig.In

	Tangle          *tangle.Tangle
	NotarizationMgr *notarization.Manager
	Storage         kvstore.KVStore
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
	deps.NotarizationMgr.Events.TangleTreeInserted.Attach(event.NewClosure(func(event *notarization.TangleTreeUpdatedEvent) {
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
	deps.NotarizationMgr.Events.StateMutationInserted.Attach(event.NewClosure(func(event *notarization.StateMutationTreeUpdatedEvent) {
		err := insertTransactionToEpoch(event.EI, event.TransactionID)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.StateMutationRemoved.Attach(event.NewClosure(func(event *notarization.StateMutationTreeUpdatedEvent) {
		err := removeTransactionFromEpoch(event.EI, event.TransactionID)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.UTXOInserted.Attach(event.NewClosure(func(event *notarization.UTXOUpdatedEvent) {
		err := insertOutputsToEpoch(event.EI, event.Spent, event.Created)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.UTXORemoved.Attach(event.NewClosure(func(event *notarization.UTXOUpdatedEvent) {
		err := removeOutputsFromEpoch(event.EI, event.Spent, event.Created)
		if err != nil {
			plugin.LogDebug(err)
		}
	}))
	deps.NotarizationMgr.Events.EpochCommitted.Attach(event.NewClosure(func(event *notarization.EpochCommittedEvent) {
		lastCommittedEpoch = event.EI
		committedEpochs[event.EI] = event.ECRecord
	}))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("EpochStorage", func(ctx context.Context) {
		<-ctx.Done()
		deps.Tangle.Shutdown()
	}, shutdown.PriorityNotarization); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func GetLastCommittedEpoch() *epoch.ECRecord {
	return committedEpochs[lastCommittedEpoch]
}

func GetCommittedEpochs() (ecRecords map[epoch.Index]*epoch.ECRecord) {
	return committedEpochs
}

func GetPendingBranchCount() map[epoch.Index]uint64 {
	return deps.NotarizationMgr.PendingConflictsCountAll()
}

func GetEpochMessages(ei epoch.Index) []tangle.MessageID {
	stores := getEpochContentStorage(ei)
	var msgIDs []tangle.MessageID

	stores.messageIDs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var msgID tangle.MessageID
		if _, err := msgID.Decode(key); err != nil {
			panic("MessageID could not be parsed!")
		}
		msgIDs = append(msgIDs, msgID)
		return true
	})

	return msgIDs
}

func GetEpochTransactions(ei epoch.Index) []utxo.TransactionID {
	stores := getEpochContentStorage(ei)
	var txIDs []utxo.TransactionID

	stores.transactionIDs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var txID utxo.TransactionID
		if _, err := txID.Decode(key); err != nil {
			panic("TransactionID could not be parsed!")
		}
		txIDs = append(txIDs, txID)
		return true
	})

	return txIDs
}

func GetEpochUTXOs(ei epoch.Index) (spent, created []utxo.OutputID) {
	stores := getEpochContentStorage(ei)
	var createdOutputs []utxo.OutputID
	var spentOutputs []utxo.OutputID

	stores.createdOutputs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		outputID, err := outputIDFromBytes(key)
		if err != nil {
			panic(err)
		}
		createdOutputs = append(createdOutputs, outputID)

		return true
	})

	stores.spentOutputs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		outputID, err := outputIDFromBytes(key)
		if err != nil {
			panic(err)
		}
		spentOutputs = append(spentOutputs, outputID)

		return true
	})

	return spentOutputs, createdOutputs
}

func getEpochContentStorage(ei epoch.Index) *epochContentStorages {
	stores, ok := epochContents[ei]
	if !ok {
		stores = newEpochContentStorage()
		epochContents[ei] = stores
	}

	return stores
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

func getEpochTransactionIDs(ei epoch.Index) ([]utxo.TransactionID, error) {
	epochContentStorage := getEpochContentStorage(ei)
	var transactionIDs []utxo.TransactionID

	_ = epochContentStorage.transactionIDs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var txID utxo.TransactionID
		if _, err := txID.Decode(key); err != nil {
			panic("TransactionID could not be parsed!")
		}
		transactionIDs = append(transactionIDs, txID)
		return true
	})
	return transactionIDs, nil
}

func getEpochMessageIDs(ei epoch.Index) ([]tangle.MessageID, error) {
	epochContentStorage := getEpochContentStorage(ei)
	var messageIDs []tangle.MessageID

	_ = epochContentStorage.messageIDs.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		var msgID tangle.MessageID
		if _, err := msgID.Decode(key); err != nil {
			panic("MessageID could not be parsed!")
		}
		messageIDs = append(messageIDs, msgID)
		return true
	})
	return messageIDs, nil
}

func insertMessageToEpoch(ei epoch.Index, msgID tangle.MessageID) error {
	epochContentStorage := getEpochContentStorage(ei)
	if err := epochContentStorage.messageIDs.Set(msgID.Bytes(), msgID.Bytes()); err != nil {
		return errors.New("Fail to insert Message to epoch store")
	}
	return nil
}

func removeMessageFromEpoch(ei epoch.Index, msgID tangle.MessageID) error {
	epochContentStorage := getEpochContentStorage(ei)
	if err := epochContentStorage.messageIDs.Delete(msgID.Bytes()); err != nil {
		return errors.New("Fail to remove Message from epoch store")
	}
	return nil
}

func insertTransactionToEpoch(ei epoch.Index, txID utxo.TransactionID) error {
	epochContentStorage := getEpochContentStorage(ei)
	if err := epochContentStorage.transactionIDs.Set(txID.Bytes(), txID.Bytes()); err != nil {
		return errors.New("Fail to insert Transaction to epoch store")
	}
	return nil
}

func removeTransactionFromEpoch(ei epoch.Index, txID utxo.TransactionID) error {
	epochContentStorage := getEpochContentStorage(ei)
	if err := epochContentStorage.transactionIDs.Delete(txID.Bytes()); err != nil {
		return errors.New("Fail to remove Transaction from epoch store")
	}
	return nil
}

func insertOutputsToEpoch(ei epoch.Index, spent, created []*ledger.OutputWithMetadata) error {
	epochContentStorage := getEpochContentStorage(ei)

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
	epochContentStorage := getEpochContentStorage(ei)

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

func outputIDFromBytes(outputBytes []byte) (utxo.OutputID, error) {
	var outputID utxo.OutputID
	if _, err := serix.DefaultAPI.Decode(context.Background(), outputBytes, outputID, serix.WithValidation()); err != nil {
		return utxo.EmptyOutputID, errors.New("Fail to parse outputID from bytes")
	}
	fmt.Println(outputID)
	return outputID, nil
}
