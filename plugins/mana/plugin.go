package mana

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/objectstorage"
)

// PluginName is the name of the mana plugin.
const (
	PluginName      = "Mana"
	manaScaleFactor = 1000 // scale floating point mana to int
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
	consensusEventsLogStorage                  *objectstorage.ObjectStorage
	consensusBaseManaPastVectorMetadataStorage *objectstorage.ObjectStorage
	onTransactionConfirmedClosure              *events.Closure
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

	allowedPledgeNodes = make(map[mana.Type]AllowedPledge)
	baseManaVectors = make(map[mana.Type]mana.BaseManaVector)
	baseManaVectors[mana.AccessMana], _ = mana.NewBaseManaVector(mana.AccessMana)
	baseManaVectors[mana.ConsensusMana], _ = mana.NewBaseManaVector(mana.ConsensusMana)
	if config.Node().GetBool(CfgManaEnableResearchVectors) {
		baseManaVectors[mana.ResearchAccess], _ = mana.NewResearchBaseManaVector(mana.WeightedMana, mana.AccessMana, mana.Mixed)
		baseManaVectors[mana.ResearchConsensus], _ = mana.NewResearchBaseManaVector(mana.WeightedMana, mana.ConsensusMana, mana.Mixed)
	}

	// configure storage for each vector type
	storages = make(map[mana.Type]*objectstorage.ObjectStorage)
	store := database.Store()
	osFactory = objectstorage.NewFactory(store, storageprefix.Mana)
	storages[mana.AccessMana] = osFactory.New(storageprefix.ManaAccess, mana.FromObjectStorage)
	storages[mana.ConsensusMana] = osFactory.New(storageprefix.ManaConsensus, mana.FromObjectStorage)
	if config.Node().GetBool(CfgManaEnableResearchVectors) {
		storages[mana.ResearchAccess] = osFactory.New(storageprefix.ManaAccessResearch, mana.FromObjectStorage)
		storages[mana.ResearchConsensus] = osFactory.New(storageprefix.ManaConsensusResearch, mana.FromObjectStorage)
	}
	consensusEventsLogStorage = osFactory.New(storageprefix.ManaEventsStorage, mana.FromEventObjectStorage)
	consensusBaseManaPastVectorStorage = osFactory.New(storageprefix.ManaConsensusPast, mana.FromObjectStorage)
	consensusBaseManaPastVectorMetadataStorage = osFactory.New(storageprefix.ManaConsensusPastMetadata, mana.FromMetadataObjectStorage)

	err := verifyPledgeNodes()
	if err != nil {
		log.Panic(err.Error())
	}

	configureEvents()
}

func configureEvents() {
	valuetransfers.Tangle().Events.TransactionConfirmed.Attach(onTransactionConfirmedClosure)
	mana.Events().Pledged.Attach(events.NewClosure(logPledgeEvent))
	mana.Events().Revoked.Attach(events.NewClosure(logRevokeEvent))
}

func logPledgeEvent(ev *mana.PledgedEvent) {
	if ev.ManaType == mana.ConsensusMana {
		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
	}
}
func logRevokeEvent(ev *mana.RevokedEvent) {
	if ev.ManaType == mana.ConsensusMana {
		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
	}
}

func onTransactionConfirmed(cachedTransactionEvent *tangle.CachedTransactionEvent) {
	cachedTransactionEvent.TransactionMetadata.Release()
	// holds all info mana pkg needs for correct mana calculations from the transaction
	var txInfo *mana.TxInfo
	// process transaction object to build txInfo
	cachedTransactionEvent.Transaction.Consume(func(tx *transaction.Transaction) {
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

			var _inputInfo mana.InputInfo
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

				// build InputInfo for this particular input in the transaction
				_inputInfo = mana.InputInfo{
					TimeStamp: inputTimestamp,
					Amount:    amount,
					PledgeID: map[mana.Type]identity.ID{
						mana.AccessMana:    accessManaNodeID,
						mana.ConsensusMana: consensusManaNodeID,
					},
					InputID: input.ID(),
				}
			})

			inputInfos = append(inputInfos, _inputInfo)
			return true
		})

		txInfo = &mana.TxInfo{
			TimeStamp:     tx.Timestamp(),
			TransactionID: tx.ID(),
			TotalBalance:  totalAmount,
			PledgeID: map[mana.Type]identity.ID{
				mana.AccessMana:    tx.AccessManaNodeID(),
				mana.ConsensusMana: tx.ConsensusManaNodeID(),
			},
			InputInfos: inputInfos,
		}
	})
	// book in all mana vectors.
	for _, baseManaVector := range baseManaVectors {
		baseManaVector.Book(txInfo)
	}
}

func run(_ *node.Plugin) {
	// mana calculation coefficients can be set from config
	ema1 := config.Node().GetFloat64(CfgEmaCoefficient1)
	ema2 := config.Node().GetFloat64(CfgEmaCoefficient2)
	dec := config.Node().GetFloat64(CfgDecay)
	mana.SetCoefficients(ema1, ema2, dec)
	if err := daemon.BackgroundWorker("Mana", func(shutdownSignal <-chan struct{}) {
		readStoredManaVectors()
		pruneStorages()
		<-shutdownSignal
		log.Info("stopping ", PluginName)
		valuetransfers.Tangle().Events.TransactionConfirmed.Detach(onTransactionConfirmedClosure)
		storeManaVectors()
		shutdownStorages()
	}, shutdown.PriorityTangle); err != nil {
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
	// TODO: causes plugin to hang

	//for vectorType := range baseManaVectors {
	//	storages[vectorType].Shutdown()
	//}
	consensusEventsLogStorage.Shutdown()
	consensusBaseManaPastVectorStorage.Shutdown()
	consensusBaseManaPastVectorMetadataStorage.Shutdown()
}

// GetHighestManaNodes returns the n highest type mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func GetHighestManaNodes(manaType mana.Type, n uint) ([]mana.Node, error) {
	bmv := baseManaVectors[manaType]
	return bmv.GetHighestManaNodes(n)
}

// GetManaMap returns type mana perception of the node.
func GetManaMap(manaType mana.Type) (mana.NodeMap, error) {
	return baseManaVectors[manaType].GetManaMap()
}

// GetAccessMana returns the access mana of the node specified.
func GetAccessMana(nodeID identity.ID) (float64, error) {
	return baseManaVectors[mana.AccessMana].GetMana(nodeID)
}

// GetConsensusMana returns the consensus mana of the node specified.
func GetConsensusMana(nodeID identity.ID) (float64, error) {
	return baseManaVectors[mana.ConsensusMana].GetMana(nodeID)
}

// GetNeighborsMana returns the type mana of the nodes neighbors
func GetNeighborsMana(manaType mana.Type) (mana.NodeMap, error) {
	neighbors := gossip.Manager().AllNeighbors()
	res := make(mana.NodeMap)
	for _, n := range neighbors {
		// in case of error, value is 0.0
		value, _ := baseManaVectors[manaType].GetMana(n.ID())
		res[n.ID()] = value
	}
	return res, nil
}

// GetAllManaMaps returns the full mana maps for comparison with the perception of other nodes.
func GetAllManaMaps() map[mana.Type]mana.NodeMap {
	res := make(map[mana.Type]mana.NodeMap)
	for manaType := range baseManaVectors {
		res[manaType], _ = GetManaMap(manaType)
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
	manaMap, _ := GetManaMap(manaType)
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
// Sorted in descending order based on mana.
func GetOnlineNodes(manaType mana.Type) ([]mana.Node, error) {
	knownPeers := autopeering.Discovery().GetVerifiedPeers()
	// consider ourselves as a peer in the network too
	knownPeers = append(knownPeers, local.GetInstance().Peer)
	onlineNodesMana := make([]mana.Node, 0)
	for _, peer := range knownPeers {
		if !baseManaVectors[manaType].Has(peer.ID()) {
			onlineNodesMana = append(onlineNodesMana, mana.Node{ID: peer.ID(), Mana: 0})
		} else {
			peerMana, err := baseManaVectors[manaType].GetMana(peer.ID())
			if err != nil {
				return nil, err
			}
			onlineNodesMana = append(onlineNodesMana, mana.Node{ID: peer.ID(), Mana: peerMana})
		}
	}
	sort.Slice(onlineNodesMana, func(i, j int) bool {
		return onlineNodesMana[i].Mana > onlineNodesMana[j].Mana
	})
	return onlineNodesMana, nil
}

func verifyPledgeNodes() error {
	access := AllowedPledge{
		IsFilterEnabled: config.Node().GetBool(CfgAllowedAccessFilterEnabled),
	}
	consensus := AllowedPledge{
		IsFilterEnabled: config.Node().GetBool(CfgAllowedConsensusFilterEnabled),
	}

	access.Allowed = set.New(false)
	// own ID is allowed by default
	access.Allowed.Add(local.GetInstance().ID())
	if access.IsFilterEnabled {
		for _, pubKey := range config.Node().GetStringSlice(CfgAllowedAccessPledge) {
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
		for _, pubKey := range config.Node().GetStringSlice(CfgAllowedConsensusPledge) {
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
func PendingManaOnOutput(outputID transaction.OutputID) float64 {
	cachedOutput := valuetransfers.Tangle().TransactionOutput(outputID)
	defer cachedOutput.Release()
	output := cachedOutput.Unwrap()
	var value float64
	for _, balance := range output.Balances() {
		value += float64(balance.Value)
	}

	// spent output has 0 pending mana.
	if output.ConsumerCount() > 0 {
		return 0
	}

	cachedTx := valuetransfers.Tangle().Transaction(output.TransactionID())
	defer cachedTx.Release()
	tx := cachedTx.Unwrap()
	return GetPendingMana(value, time.Since(tx.Timestamp()))
}

// GetPendingMana returns the mana pledged by spending a `value` output that sat for `n` duration.
func GetPendingMana(value float64, n time.Duration) float64 {
	return value * (1 - math.Pow(math.E, -mana.Decay*(n.Seconds())))
}

// GetLoggedEvents gets the events logs for the node IDs specified. If none is specified, it returns the logs for all nodes.
func GetLoggedEvents(IDs []identity.ID) (map[identity.ID]*EventsLogs, error) {
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

	return logs, err
}

// GetPastConsensusManaVectorMetadata gets the past consensus mana vector metadata.
func GetPastConsensusManaVectorMetadata() mana.ConsensusBasePastManaVectorMetadata {
	cachedObj := consensusBaseManaPastVectorMetadataStorage.Get([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
	defer cachedMetadata.Release()
	metadata := cachedMetadata.Unwrap()
	return *metadata
}

// GetPastConsensusManaVector builds a consensus base mana vector in the past.
func GetPastConsensusManaVector(t time.Time) (*mana.ConsensusBaseManaVector, error) {
	baseManaVector, err := mana.NewBaseManaVector(mana.ConsensusMana)
	if err != nil {
		return nil, err
	}
	cbmvPast := baseManaVector.(*mana.ConsensusBaseManaVector)
	cachedObj := consensusBaseManaPastVectorMetadataStorage.Get([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
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

	var eventLogs []mana.Event
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
		return nil, err
	}
	sort.Slice(eventLogs, func(i, j int) bool {
		var timeI, timeJ time.Time
		var typeI, _ byte
		switch eventLogs[i].Type() {
		case mana.EventTypePledge:
			timeI = eventLogs[i].(*mana.PledgedEvent).Time
			typeI = mana.EventTypePledge
		case mana.EventTypeRevoke:
			timeI = eventLogs[i].(*mana.RevokedEvent).Time
			typeI = mana.EventTypeRevoke
		}

		switch eventLogs[j].Type() {
		case mana.EventTypePledge:
			timeJ = eventLogs[j].(*mana.PledgedEvent).Time
			_ = mana.EventTypePledge
		case mana.EventTypeRevoke:
			timeJ = eventLogs[j].(*mana.RevokedEvent).Time
			_ = mana.EventTypeRevoke
		}

		if !timeI.Equal(timeJ) {
			return timeI.Before(timeJ)
		}

		return typeI == mana.EventTypeRevoke
	})
	consumed, err := cbmvPast.BuildPastBaseVector(eventLogs, t)
	if err != nil {
		return nil, err
	}

	// store the most recent bmv
	if err = consensusBaseManaPastVectorStorage.Prune(); err != nil {
		return nil, err
	}
	for _, p := range cbmvPast.ToPersistables() {
		consensusBaseManaPastVectorStorage.Store(p).Release()
	}

	err = cbmvPast.UpdateAll(t)
	if err != nil {
		return nil, err
	}

	//store the metadata
	metadata := &mana.ConsensusBasePastManaVectorMetadata{
		Timestamp: t,
		Consumed:  int64(consumed),
	}

	if !cachedMetadata.Exists() {
		consensusBaseManaPastVectorMetadataStorage.Store(metadata).Release()
	} else {
		m := cachedMetadata.Unwrap()
		m.Update(metadata)
	}

	// TODO: delete logs? what if we want to go further in the past
	//consensusEventLogs = consensusEventLogs[0:consumedLogs]
	return cbmvPast, err
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
