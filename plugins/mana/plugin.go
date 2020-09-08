package mana

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/objectstorage"
)

// TODO: expose plugin functions to the outside

//GetHighestManaNodes(type, n) [n]NodeIdManaTuple: return the n highest type mana nodes (nodeID,manaValue) in ascending order. Should also update their mana value.
//GetManaMap(type) map[nodeID]manaValue: return type mana perception of the node.
//GetAccessMana(nodeID) mana: access Base Mana Vector of Access Mana, update its values with respect to time, and return the amount of Access Mana (either Effective Base Mana 1, Effective Base Mana 2, or some combination of the two). Trigger ManaUpdated event.
//GetConsensusMana(nodeID) mana: access Base Mana Vector of Consensus Mana, update its values with respect to time, and returns the amount of Consensus Mana (either Effective Base Mana 1, Effective Base Mana 2, or some combination of the two). Trigger ManaUpdated event.
//GetNeighborsMana(type): returns the type mana of the nodes neighbors
//GetAllManaVectors() Obtaining the full mana maps for comparison with the perception of other nodes.
//GetWeightedRandomNodes(n): returns a weighted random selection of n nodes. Consensus Mana is used for the weights.
//Obtaining a list of currently known peers + their mana, sorted. Useful for knowing which high mana nodes are online.
//OverrideMana(nodeID, baseManaVector): Sets the nodes mana to a specific value. Can be useful for debugging, setting faucet mana, initialization, etc.. Triggers ManaUpdated

// PluginName is the name of the mana plugin.
const (
	PluginName = "Mana"
)

var (
	// plugin is the plugin instance of the mana plugin.
	plugin          *node.Plugin
	once            sync.Once
	log             *logger.Logger
	baseManaVectors map[mana.Type]*mana.BaseManaVector
	osFactory       *objectstorage.Factory
	storages        map[mana.Type]*objectstorage.ObjectStorage
)

func osManaFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return mana.FromStorageKey(key)
}

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	baseManaVectors = make(map[mana.Type]*mana.BaseManaVector)
	baseManaVectors[mana.AccessMana] = mana.NewBaseManaVector(mana.AccessMana)
	baseManaVectors[mana.ConsensusMana] = mana.NewBaseManaVector(mana.ConsensusMana)

	// configure storage for each vector type
	storages = make(map[mana.Type]*objectstorage.ObjectStorage)
	store := database.Store()
	osFactory = objectstorage.NewFactory(store, storageprefix.Mana)
	storages[mana.AccessMana] = osFactory.New(storageprefix.Mana, osManaFactory)
	storages[mana.ConsensusMana] = osFactory.New(storageprefix.Mana, osManaFactory)

	configureEvents()
}

func configureEvents() {
	valuetransfers.Tangle().Events.TransactionConfirmed.Attach(events.NewClosure(func(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()
		// holds all info mana pkg needs for correct mana calculations from the transaction
		var txInfo *mana.TxInfo
		// process transaction object to build txInfo
		cachedTransaction.Consume(func(tx *transaction.Transaction) {
			var totalAmount float64
			var inputInfos []mana.InputInfo
			// iterate over all inputs within the transaction
			tx.Inputs().ForEach(func(inputID transaction.OutputID) bool {
				var amount float64
				var inputTimestamp time.Time
				var accessManaNodeID identity.ID
				var consensusManaNodeID identity.ID
				// get output object from storage
				cachedInput := valuetransfers.Tangle().TransactionOutput(inputID)
				// process it to be able to build an InputInfo struct
				cachedInput.Consume(func(input *tangle.Output) {
					// first, sum balances of the input, calculate total amount as well for later
					for _, inputBalance := range input.Balances() {
						amount += float64(inputBalance.Value)
						totalAmount += amount
					}
					// derive the transaction that created this input
					cachedInputTx := valuetransfers.Tangle().Transaction(input.TransactionID())
					// look into the transaction, we need timestamp and access & consensus pledge IDs
					cachedInputTx.Consume(func(inputTx *transaction.Transaction) {
						if inputTx != nil {
							inputTimestamp = inputTx.Timestamp()
							accessManaNodeID = inputTx.AccessManaNodeID()
							consensusManaNodeID = inputTx.ConsensusManaNodeID()
						}
					})
				})

				// build InputInfo for this particular input in the transaction
				_inputInfo := mana.InputInfo{
					TimeStamp: inputTimestamp,
					Amount:    amount,
					PledgeID: map[mana.Type]identity.ID{
						mana.AccessMana:    accessManaNodeID,
						mana.ConsensusMana: consensusManaNodeID,
					},
				}

				inputInfos = append(inputInfos, _inputInfo)
				return true
			})

			txInfo = &mana.TxInfo{
				TimeStamp:    tx.Timestamp(),
				TotalBalance: totalAmount,
				PledgeID: map[mana.Type]identity.ID{
					mana.AccessMana:    tx.AccessManaNodeID(),
					mana.ConsensusMana: tx.ConsensusManaNodeID(),
				},
				InputInfos: inputInfos,
			}
		})
		// book in all mana vectors.
		for _, baseManaVector := range baseManaVectors {
			baseManaVector.BookMana(txInfo)
		}
	}))
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Mana", func(shutdownSignal <-chan struct{}) {
		readStoredManaVectors()
		pruneStroages()
		<-shutdownSignal
		storeManaVectors()
		shutdownStorages()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func readStoredManaVectors() {
	vectorTypes := []mana.Type{mana.AccessMana, mana.ConsensusMana}
	for _, vectorType := range vectorTypes {
		storages[vectorType].ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
			cachedPbm := &mana.CachedPersistableBaseMana{CachedObject: cachedObject}
			cachedPbm.Consume(func(p *mana.PersistableBaseMana) {
				baseManaVectors[vectorType].FromPersitable(p)
			})
			return true
		})
	}
}

func storeManaVectors() {
	for vectorType, baseManaVector := range baseManaVectors {
		persitables := baseManaVector.ToPersistables()
		for _, p := range persitables {
			storages[vectorType].Store(p).Release()
		}
	}
}

func pruneStorages() {
	for vectorType := range baseManaVectors {
		_ = storages[vectorType].Prune()
	}
}

func shutdownStorages() {
	for vectorType := range baseManaVectors {
		storages[vectorType].Shutdown()
	}
}
