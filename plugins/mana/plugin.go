package mana

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/mana"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
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
const PluginName = "Mana"

var (
	// plugin is the plugin instance of the mana plugin.
	plugin          *node.Plugin
	once            sync.Once
	log             *logger.Logger
	baseManaVectors map[mana.Type]*mana.BaseManaVector
)

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

	configureEvents()
}

func configureEvents() {
	valuetransfers.Tangle().Events.TransactionConfirmed.Attach(events.NewClosure(func(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()
		cachedTransaction.Consume(func(tx *transaction.Transaction) {
			var inputInfo []mana.InputInfo
			tx.Inputs().ForEach(func(inputID transaction.OutputID) bool {
				var amount float64
				cachedInput := valuetransfers.Tangle().TransactionOutput(inputID)
				cachedInput.Consume(func(input *tangle.Output) {
					for _, inputBalance := range input.Balances() {
						amount += float64(inputBalance.Value)
					}
				})

				_inputInfo := mana.InputInfo{
					TimeStamp:         valuetransfers.Tangle().GetTimestamp(inputID.TransactionID()),
					Amount:            amount,
					AccessPledgeID:    valuetransfers.Tangle().GetAccessManaNodeID(inputID.TransactionID()),
					ConsensusPledgeID: valuetransfers.Tangle().GetConsensusManaNodeID(inputID.TransactionID()),
				}
				inputInfo = append(inputInfo, _inputInfo)
				return true
			})

			txInfo := &mana.TxInfo{
				TimeStamp: tx.Timestamp(),
				InputInfo: inputInfo,
			}

			for _, baseManaVector := range baseManaVectors {
				baseManaVector.BookMana(txInfo)
			}
		})
	}))
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Mana", func(shutdownSignal <-chan struct{}) {
		// TODO: Read base mana vectors from storage
		<-shutdownSignal
		// TODO: write base mana vectors to object storage
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
