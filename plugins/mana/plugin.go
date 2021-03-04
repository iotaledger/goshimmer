package mana

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	db_pkg "github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/objectstorage"
	"go.uber.org/atomic"
)

// PluginName is the name of the mana plugin.
const (
	PluginName                  = "Mana"
	manaScaleFactor             = 1000 // scale floating point mana to int
	maxConsensusEventsInStorage = 108000
	slidingEventsInterval       = 10800 //10% of maxConsensusEventsInStorage
)

var (
	// plugin is the plugin instance of the mana plugin.
	plugin                                     *node.Plugin
	once                                       sync.Once
	log                                        *logger.Logger
	baseManaVectors                            map[mana.Type]mana.BaseManaVector
	osFactory                                  *objectstorage.Factory
	storages                                   map[mana.Type]*objectstorage.ObjectStorage
	allowedPledgeNodes                         map[mana.Type]AllowedPledge
	consensusBaseManaPastVectorStorage         *objectstorage.ObjectStorage
	consensusBaseManaPastVectorMetadataStorage *objectstorage.ObjectStorage
	consensusEventsLogStorage                  *objectstorage.ObjectStorage
	consensusEventsLogsStorageSize             atomic.Uint32
	onTransactionConfirmedClosure              *events.Closure
	onPledgeEventClosure                       *events.Closure
	onRevokeEventClosure                       *events.Closure
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

	onTransactionConfirmedClosure = events.NewClosure(onTransactionConfirmed)
	onPledgeEventClosure = events.NewClosure(logPledgeEvent)
	onRevokeEventClosure = events.NewClosure(logRevokeEvent)

	allowedPledgeNodes = make(map[mana.Type]AllowedPledge)
	baseManaVectors = make(map[mana.Type]mana.BaseManaVector)
	baseManaVectors[mana.AccessMana], _ = mana.NewBaseManaVector(mana.AccessMana)
	baseManaVectors[mana.ConsensusMana], _ = mana.NewBaseManaVector(mana.ConsensusMana)
	if config.Node().Bool(CfgManaEnableResearchVectors) {
		baseManaVectors[mana.ResearchAccess], _ = mana.NewResearchBaseManaVector(mana.WeightedMana, mana.AccessMana, mana.Mixed)
		baseManaVectors[mana.ResearchConsensus], _ = mana.NewResearchBaseManaVector(mana.WeightedMana, mana.ConsensusMana, mana.Mixed)
	}

	// configure storage for each vector type
	storages = make(map[mana.Type]*objectstorage.ObjectStorage)
	store := database.Store()
	osFactory = objectstorage.NewFactory(store, db_pkg.PrefixMana)
	storages[mana.AccessMana] = osFactory.New(mana.PrefixAccess, mana.FromObjectStorage)
	storages[mana.ConsensusMana] = osFactory.New(mana.PrefixConsensus, mana.FromObjectStorage)
	if config.Node().Bool(CfgManaEnableResearchVectors) {
		storages[mana.ResearchAccess] = osFactory.New(mana.PrefixAccessResearch, mana.FromObjectStorage)
		storages[mana.ResearchConsensus] = osFactory.New(mana.PrefixConsensusResearch, mana.FromObjectStorage)
	}
	consensusEventsLogStorage = osFactory.New(mana.PrefixEventStorage, mana.FromEventObjectStorage)
	consensusEventsLogsStorageSize.Store(getConsensusEventLogsStorageSize())
	consensusBaseManaPastVectorStorage = osFactory.New(mana.PrefixConsensusPastVector, mana.FromObjectStorage)
	consensusBaseManaPastVectorMetadataStorage = osFactory.New(mana.PrefixConsensusPastMetadata, mana.FromMetadataObjectStorage)

	err := verifyPledgeNodes()
	if err != nil {
		log.Panic(err.Error())
	}

	configureEvents()
}

func configureEvents() {
	// until we have the proper event...
	messagelayer.Tangle().OpinionFormer.Events.TransactionConfirmed.Attach(onTransactionConfirmedClosure)
	mana.Events().Pledged.Attach(onPledgeEventClosure)
	mana.Events().Revoked.Attach(onRevokeEventClosure)
}

func logPledgeEvent(ev *mana.PledgedEvent) {
	if ev.ManaType == mana.ConsensusMana {
		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
		consensusEventsLogsStorageSize.Inc()
	}
}
func logRevokeEvent(ev *mana.RevokedEvent) {
	if ev.ManaType == mana.ConsensusMana {
		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
		consensusEventsLogsStorageSize.Inc()
	}
}

func onTransactionConfirmed(msgID tangle.MessageID) {
	var tx *ledgerstate.Transaction
	isTx := false
	messagelayer.Tangle().Storage.Message(msgID).Consume(func(message *tangle.Message) {
		if message.Payload().Type() != ledgerstate.TransactionType {
			return
		}
		isTx = true
		var err error
		tx, _, err = ledgerstate.TransactionFromBytes(message.Payload().Bytes())
		if err != nil {
			isTx = false
			log.Errorf("Message %s contains invalid transaction payload: %w", msgID.String(), err)
			return
		}
	})
	if !isTx {
		return
	}

	// holds all info mana pkg needs for correct mana calculations from the transaction
	var txInfo *mana.TxInfo
	// process transaction object to build txInfo
	var totalAmount float64
	var inputInfos []mana.InputInfo

	// iterate over all inputs within the transaction
	for _, input := range tx.Essence().Inputs() {
		i := input.(*ledgerstate.UTXOInput)

		var amount float64
		var inputTimestamp time.Time
		var accessManaNodeID identity.ID
		var consensusManaNodeID identity.ID
		var _inputInfo mana.InputInfo

		messagelayer.Tangle().LedgerState.Output(i.ReferencedOutputID()).Consume(func(o ledgerstate.Output) {
			// first, sum balances of the input, calculate total amount as well for later
			o.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				amount += float64(balance)
				totalAmount += amount
				return true
			})
			// derive the transaction that created this input
			inputTxID := o.ID().TransactionID()
			// look into the transaction, we need timestamp and access & consensus pledge IDs
			messagelayer.Tangle().LedgerState.Transaction(inputTxID).Consume(func(transaction *ledgerstate.Transaction) {
				if transaction != nil {
					inputTimestamp = transaction.Essence().Timestamp()
					accessManaNodeID = transaction.Essence().AccessPledgeID()
					consensusManaNodeID = transaction.Essence().ConsensusPledgeID()
				}
			})
			// build InputInfo for this particular input in the transaction
			_inputInfo = mana.InputInfo{
				TimeStamp: inputTimestamp,
				Amount:    amount,
				PledgeID: map[mana.Type]identity.ID{
					mana.AccessMana:    accessManaNodeID,
					mana.ConsensusMana: consensusManaNodeID,
				},
				InputID: o.ID(),
			}
		})
		inputInfos = append(inputInfos, _inputInfo)
	}

	txInfo = &mana.TxInfo{
		TimeStamp:     tx.Essence().Timestamp(),
		TransactionID: tx.ID(),
		TotalBalance:  totalAmount,
		PledgeID: map[mana.Type]identity.ID{
			mana.AccessMana:    tx.Essence().AccessPledgeID(),
			mana.ConsensusMana: tx.Essence().ConsensusPledgeID(),
		},
		InputInfos: inputInfos,
	}
	// book in all mana vectors.
	for _, baseManaVector := range baseManaVectors {
		baseManaVector.Book(txInfo)
	}
}

func run(_ *node.Plugin) {
	// mana calculation coefficients can be set from config
	ema1 := config.Node().Float64(CfgEmaCoefficient1)
	ema2 := config.Node().Float64(CfgEmaCoefficient2)
	dec := config.Node().Float64(CfgDecay)
	pruneInterval := config.Node().Duration(CfgPruneConsensusEventLogsInterval)

	mana.SetCoefficients(ema1, ema2, dec)
	if err := daemon.BackgroundWorker("Mana", func(shutdownSignal <-chan struct{}) {
		defer log.Infof("Stopping %s ... done", PluginName)
		ticker := time.NewTicker(pruneInterval)
		defer ticker.Stop()
		readStoredManaVectors()
		pruneStorages()
		for {
			select {
			case <-shutdownSignal:
				log.Infof("Stopping %s ...", PluginName)
				mana.Events().Pledged.Detach(onPledgeEventClosure)
				mana.Events().Pledged.Detach(onRevokeEventClosure)
				messagelayer.Tangle().OpinionFormer.Events.TransactionConfirmed.Detach(onTransactionConfirmedClosure)
				storeManaVectors()
				shutdownStorages()
				return
			case <-ticker.C:
				pruneConsensusEventLogsStorage()
			}
		}
	}, shutdown.PriorityMana); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func readStoredManaVectors() {
	for vectorType := range baseManaVectors {
		storages[vectorType].ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
			cachedPbm := &mana.CachedPersistableBaseMana{CachedObject: cachedObject}
			cachedPbm.Consume(func(p *mana.PersistableBaseMana) {
				err := baseManaVectors[vectorType].FromPersistable(p)
				if err != nil {
					log.Errorf("error while restoring %s mana vector: %w", vectorType.String(), err)
				}
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
	// TODO: does it still cause the plugin to hang?
	for vectorType := range baseManaVectors {
		storages[vectorType].Shutdown()
	}
	consensusEventsLogStorage.Shutdown()
	consensusBaseManaPastVectorStorage.Shutdown()
	consensusBaseManaPastVectorMetadataStorage.Shutdown()
}

// GetHighestManaNodes returns the n highest type mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func GetHighestManaNodes(manaType mana.Type, n uint) ([]mana.Node, time.Time, error) {
	bmv := baseManaVectors[manaType]
	return bmv.GetHighestManaNodes(n)
}

// GetManaMap returns type mana perception of the node.
func GetManaMap(manaType mana.Type, optionalUpdateTime ...time.Time) (mana.NodeMap, time.Time, error) {
	return baseManaVectors[manaType].GetManaMap(optionalUpdateTime...)
}

// GetAccessMana returns the access mana of the node specified.
func GetAccessMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	return baseManaVectors[mana.AccessMana].GetMana(nodeID, optionalUpdateTime...)
}

// GetConsensusMana returns the consensus mana of the node specified.
func GetConsensusMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	return baseManaVectors[mana.ConsensusMana].GetMana(nodeID, optionalUpdateTime...)
}

// GetNeighborsMana returns the type mana of the nodes neighbors
func GetNeighborsMana(manaType mana.Type, optionalUpdateTime ...time.Time) (mana.NodeMap, error) {
	neighbors := gossip.Manager().AllNeighbors()
	res := make(mana.NodeMap)
	for _, n := range neighbors {
		// in case of error, value is 0.0
		value, _, _ := baseManaVectors[manaType].GetMana(n.ID(), optionalUpdateTime...)
		res[n.ID()] = value
	}
	return res, nil
}

// GetAllManaMaps returns the full mana maps for comparison with the perception of other nodes.
func GetAllManaMaps(optionalUpdateTime ...time.Time) map[mana.Type]mana.NodeMap {
	res := make(map[mana.Type]mana.NodeMap)
	for manaType := range baseManaVectors {
		res[manaType], _, _ = GetManaMap(manaType, optionalUpdateTime...)
	}
	return res
}

// OverrideMana sets the nodes mana to a specific value.
// It can be useful for debugging, setting faucet mana, initialization, etc.. Triggers ManaUpdated
func OverrideMana(manaType mana.Type, nodeID identity.ID, bm *mana.AccessBaseMana) {
	baseManaVectors[manaType].SetMana(nodeID, bm)
}

//GetWeightedRandomNodes returns a weighted random selection of n nodes.
func GetWeightedRandomNodes(n uint, manaType mana.Type) mana.NodeMap {
	rand.Seed(time.Now().UTC().UnixNano())
	manaMap, _, _ := GetManaMap(manaType)
	var choices []mana.RandChoice
	for nodeID, manaValue := range manaMap {
		choices = append(choices, mana.RandChoice{
			Item:   nodeID,
			Weight: int(manaValue * manaScaleFactor), //scale float mana to int
		})
	}
	chooser := mana.NewRandChooser(choices...)
	pickedNodes := chooser.Pick(n)
	res := make(mana.NodeMap)
	for _, nodeID := range pickedNodes {
		ID := nodeID.(identity.ID)
		res[ID] = manaMap[ID]
	}
	return res
}

// GetAllowedPledgeNodes returns the list of nodes that type mana is allowed to be pledged to.
func GetAllowedPledgeNodes(manaType mana.Type) AllowedPledge {
	return allowedPledgeNodes[manaType]
}

// GetOnlineNodes gets the list of currently known (and verified) peers in the network, and their respective mana values.
// Sorted in descending order based on mana. Zero mana nodes are excluded.
func GetOnlineNodes(manaType mana.Type) (onlineNodesMana []mana.Node, t time.Time, err error) {
	knownPeers := autopeering.Discovery().GetVerifiedPeers()
	// consider ourselves as a peer in the network too
	knownPeers = append(knownPeers, local.GetInstance().Peer)
	onlineNodesMana = make([]mana.Node, 0)
	for _, peer := range knownPeers {
		if baseManaVectors[manaType].Has(peer.ID()) {
			var peerMana float64
			peerMana, t, err = baseManaVectors[manaType].GetMana(peer.ID())
			if err != nil {
				return nil, t, err
			}
			if peerMana > 0 {
				onlineNodesMana = append(onlineNodesMana, mana.Node{ID: peer.ID(), Mana: peerMana})
			}
		}
	}
	sort.Slice(onlineNodesMana, func(i, j int) bool {
		return onlineNodesMana[i].Mana > onlineNodesMana[j].Mana
	})
	return
}

func verifyPledgeNodes() error {
	access := AllowedPledge{
		IsFilterEnabled: config.Node().Bool(CfgAllowedAccessFilterEnabled),
	}
	consensus := AllowedPledge{
		IsFilterEnabled: config.Node().Bool(CfgAllowedConsensusFilterEnabled),
	}

	access.Allowed = set.New(false)
	// own ID is allowed by default
	access.Allowed.Add(local.GetInstance().ID())
	if access.IsFilterEnabled {
		for _, pubKey := range config.Node().Strings(CfgAllowedAccessPledge) {
			ID, err := mana.IDFromStr(pubKey)
			if err != nil {
				return err
			}
			access.Allowed.Add(ID)
		}
	}

	consensus.Allowed = set.New(false)
	// own ID is allowed by default
	consensus.Allowed.Add(local.GetInstance().ID())
	if consensus.IsFilterEnabled {
		for _, pubKey := range config.Node().Strings(CfgAllowedConsensusPledge) {
			ID, err := mana.IDFromStr(pubKey)
			if err != nil {
				return err
			}
			consensus.Allowed.Add(ID)
		}
	}

	allowedPledgeNodes[mana.AccessMana] = access
	allowedPledgeNodes[mana.ConsensusMana] = consensus
	return nil
}

// PendingManaOnOutput predicts how much mana (bm2) will be pledged to a node if the output specified is spent.
func PendingManaOnOutput(outputID ledgerstate.OutputID) (float64, time.Time) {
	cachedOutputMetadata := messagelayer.Tangle().LedgerState.OutputMetadata(outputID)
	defer cachedOutputMetadata.Release()
	outputMetadata := cachedOutputMetadata.Unwrap()

	// spent output has 0 pending mana.
	if outputMetadata.ConsumerCount() > 0 {
		return 0, time.Time{}
	}

	var value float64
	messagelayer.Tangle().LedgerState.Output(outputID).Consume(func(output ledgerstate.Output) {
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			value += float64(balance)
			return true
		})
	})

	cachedTx := messagelayer.Tangle().LedgerState.Transaction(outputID.TransactionID())
	defer cachedTx.Release()
	tx := cachedTx.Unwrap()
	txTimestamp := tx.Essence().Timestamp()
	return GetPendingMana(value, time.Since(txTimestamp)), txTimestamp
}

// GetPendingMana returns the mana pledged by spending a `value` output that sat for `n` duration.
func GetPendingMana(value float64, n time.Duration) float64 {
	return value * (1 - math.Pow(math.E, -mana.Decay*(n.Seconds())))
}

// GetLoggedEvents gets the events logs for the node IDs and time frame specified. If none is specified, it returns the logs for all nodes.
func GetLoggedEvents(IDs []identity.ID, startTime time.Time, endTime time.Time) (map[identity.ID]*EventsLogs, error) {
	logs := make(map[identity.ID]*EventsLogs)
	lookup := make(map[identity.ID]bool)
	getAll := true

	if len(IDs) > 0 {
		getAll = false
		for _, nodeID := range IDs {
			lookup[nodeID] = true
		}
	}

	var err error
	consensusEventsLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedPe := &mana.CachedPersistableEvent{CachedObject: cachedObject}
		defer cachedPe.Release()
		pbm := cachedPe.Unwrap()

		if !getAll {
			if !lookup[pbm.NodeID] {
				return true
			}
		}

		if _, found := logs[pbm.NodeID]; !found {
			logs[pbm.NodeID] = &EventsLogs{}
		}

		var ev mana.Event
		ev, err = mana.FromPersistableEvent(pbm)
		if err != nil {
			return false
		}

		if ev.Timestamp().Before(startTime) || ev.Timestamp().After(endTime) {
			return true
		}
		switch ev.Type() {
		case mana.EventTypePledge:
			logs[pbm.NodeID].Pledge = append(logs[pbm.NodeID].Pledge, ev.(*mana.PledgedEvent))
		case mana.EventTypeRevoke:
			logs[pbm.NodeID].Revoke = append(logs[pbm.NodeID].Revoke, ev.(*mana.RevokedEvent))
		default:
			err = mana.ErrUnknownManaEvent
			return false
		}
		return true
	})

	for ID := range logs {
		sort.Slice(logs[ID].Pledge, func(i, j int) bool {
			return logs[ID].Pledge[i].Time.Before(logs[ID].Pledge[j].Time)
		})
		sort.Slice(logs[ID].Revoke, func(i, j int) bool {
			return logs[ID].Revoke[i].Time.Before(logs[ID].Revoke[j].Time)
		})
	}

	return logs, err
}

// GetPastConsensusManaVectorMetadata gets the past consensus mana vector metadata.
func GetPastConsensusManaVectorMetadata() *mana.ConsensusBasePastManaVectorMetadata {
	cachedObj := consensusBaseManaPastVectorMetadataStorage.Load([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
	defer cachedMetadata.Release()
	return cachedMetadata.Unwrap()
}

// GetPastConsensusManaVector builds a consensus base mana vector in the past.
func GetPastConsensusManaVector(t time.Time) (*mana.ConsensusBaseManaVector, []mana.Event, error) {
	baseManaVector, err := mana.NewBaseManaVector(mana.ConsensusMana)
	if err != nil {
		return nil, nil, err
	}
	cbmvPast := baseManaVector.(*mana.ConsensusBaseManaVector)
	cachedObj := consensusBaseManaPastVectorMetadataStorage.Load([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
	defer cachedMetadata.Release()

	if cachedMetadata.Exists() {
		metadata := cachedMetadata.Unwrap()
		if t.After(metadata.Timestamp) {
			consensusBaseManaPastVectorStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
				cachedPbm := &mana.CachedPersistableBaseMana{CachedObject: cachedObject}
				defer cachedPbm.Release()
				p := cachedPbm.Unwrap()
				err = cbmvPast.FromPersistable(p)
				if err != nil {
					log.Errorf("error while restoring %s mana vector from storage: %w", mana.ConsensusMana.String(), err)
					baseManaVector, _ := mana.NewBaseManaVector(mana.ConsensusMana)
					cbmvPast = baseManaVector.(*mana.ConsensusBaseManaVector)
					return false
				}
				return true
			})
		}
	}

	var eventLogs mana.EventSlice
	consensusEventsLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedPe := &mana.CachedPersistableEvent{CachedObject: cachedObject}
		defer cachedPe.Release()
		pe := cachedPe.Unwrap()
		if pe.Time.After(t) {
			return true
		}

		// already consumed in stored base mana vector.
		if cachedMetadata.Exists() && cbmvPast.Size() > 0 {
			metadata := cachedMetadata.Unwrap()
			if pe.Time.Before(metadata.Timestamp) {
				return true
			}
		}

		var ev mana.Event
		ev, err = mana.FromPersistableEvent(pe)
		if err != nil {
			return false
		}
		eventLogs = append(eventLogs, ev)
		return true
	})
	if err != nil {
		return nil, nil, err
	}
	eventLogs.Sort()
	err = cbmvPast.BuildPastBaseVector(eventLogs, t)
	if err != nil {
		return nil, nil, err
	}

	err = cbmvPast.UpdateAll(t)
	if err != nil {
		return nil, nil, err
	}

	return cbmvPast, eventLogs, nil
}

func getConsensusEventLogsStorageSize() uint32 {
	var size uint32
	consensusEventsLogStorage.ForEachKeyOnly(func(key []byte) bool {
		size++
		return true
	}, true)
	return size
}

func pruneConsensusEventLogsStorage() {
	if consensusEventsLogsStorageSize.Load() < maxConsensusEventsInStorage {
		return
	}
	cachedObj := consensusBaseManaPastVectorMetadataStorage.Get([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
	defer cachedMetadata.Release()

	bmv, err := mana.NewBaseManaVector(mana.ConsensusMana)
	if err != nil {
		log.Errorf("error creating consensus base mana vector: %v", err)
		return
	}
	cbmvPast := bmv.(*mana.ConsensusBaseManaVector)
	if cachedMetadata.Exists() {
		consensusBaseManaPastVectorStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
			cachedPbm := &mana.CachedPersistableBaseMana{CachedObject: cachedObject}
			pbm := cachedPbm.Unwrap()
			if pbm != nil {
				err = cbmvPast.FromPersistable(pbm)
				if err != nil {
					return false
				}
			}
			return true
		})
		if err != nil {
			log.Errorf("error reading stored consensus base mana vector: %v", err)
			return
		}
	}

	var eventLogs mana.EventSlice
	consensusEventsLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedPe := &mana.CachedPersistableEvent{CachedObject: cachedObject}
		defer cachedPe.Release()
		pe := cachedPe.Unwrap()

		var ev mana.Event
		ev, err = mana.FromPersistableEvent(pe)
		if err != nil {
			return false
		}
		eventLogs = append(eventLogs, ev)
		return true
	})
	if err != nil {
		log.Infof("error reading persistable events: %v", err)
		return
	}
	eventLogs.Sort()
	// Make sure to take related events.
	// Ensures that related events (same time) are not split between different intervals.
	prev := eventLogs[slidingEventsInterval-1]
	var i int
	for i = slidingEventsInterval; i < len(eventLogs); i++ {
		if eventLogs[i].Timestamp() != prev.Timestamp() {
			break
		}
		prev = eventLogs[i]
	}
	eventLogs = eventLogs[:i]
	t := eventLogs[len(eventLogs)-1].Timestamp()

	err = cbmvPast.BuildPastBaseVector(eventLogs, t)
	if err != nil {
		log.Error("error building past consensus base mana vector: %v", err)
		return
	}

	// store cbmv
	if err = consensusBaseManaPastVectorStorage.Prune(); err != nil {
		log.Errorf("error pruning consensus base mana vector storage: %w", err)
		return
	}
	for _, p := range cbmvPast.ToPersistables() {
		consensusBaseManaPastVectorStorage.Store(p).Release()
	}

	//store the metadata
	metadata := &mana.ConsensusBasePastManaVectorMetadata{
		Timestamp: t,
	}

	if !cachedMetadata.Exists() {
		consensusBaseManaPastVectorMetadataStorage.Store(metadata).Release()
	} else {
		m := cachedMetadata.Unwrap()
		m.Update(metadata)
	}

	var entriesToDelete [][]byte
	for _, ev := range eventLogs {
		entriesToDelete = append(entriesToDelete, ev.ToPersistable().ObjectStorageKey())
	}
	consensusEventsLogStorage.DeleteEntriesFromStore(entriesToDelete)
	consensusEventsLogsStorageSize.Sub(uint32(len(entriesToDelete)))
}

// AllowedPledge represents the nodes that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool
	Allowed         set.Set
}

// EventsLogs represents the events logs.
type EventsLogs struct {
	Pledge []*mana.PledgedEvent `json:"pledge"`
	Revoke []*mana.RevokedEvent `json:"revoke"`
}
