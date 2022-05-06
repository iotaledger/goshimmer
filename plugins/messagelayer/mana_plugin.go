package messagelayer

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	db_pkg "github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	// PluginName is the name of the mana plugin.
	PluginName = "Mana"
)

var (
	// ManaPlugin is the plugin instance of the mana plugin.
	ManaPlugin         = node.NewPlugin(PluginName, nil, node.Enabled, configureManaPlugin, runManaPlugin)
	manaLogger         *logger.Logger
	baseManaVectors    map[mana.Type]mana.BaseManaVector
	storages           map[mana.Type]*objectstorage.ObjectStorage[*mana.PersistableBaseMana]
	allowedPledgeNodes map[mana.Type]AllowedPledge
	// consensusBaseManaPastVectorStorage         *objectstorage.ObjectStorage
	// consensusBaseManaPastVectorMetadataStorage *objectstorage.ObjectStorage
	// consensusEventsLogStorage                  *objectstorage.ObjectStorage
	// consensusEventsLogsStorageSize             atomic.Uint32.
	onTransactionConfirmedClosure *events.Closure
	// onPledgeEventClosure          *events.Closure
	// onRevokeEventClosure          *events.Closure
	// debuggingEnabled              bool.
)

func init() {
	ManaPlugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
		if err := event.Container.Provide(func() mana.ManaRetrievalFunc {
			return GetConsensusMana
		}, dig.Name("manaFunc")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configureManaPlugin(*node.Plugin) {
	manaLogger = logger.NewLogger(PluginName)

	onTransactionConfirmedClosure = events.NewClosure(onTransactionConfirmed)
	// onPledgeEventClosure = events.NewClosure(logPledgeEvent)
	// onRevokeEventClosure = events.NewClosure(logRevokeEvent)

	allowedPledgeNodes = make(map[mana.Type]AllowedPledge)
	baseManaVectors = make(map[mana.Type]mana.BaseManaVector)
	baseManaVectors[mana.AccessMana], _ = mana.NewBaseManaVector(mana.AccessMana)
	baseManaVectors[mana.ConsensusMana], _ = mana.NewBaseManaVector(mana.ConsensusMana)

	// configure storage for each vector type
	storages = make(map[mana.Type]*objectstorage.ObjectStorage[*mana.PersistableBaseMana])
	store := deps.Storage
	storages[mana.AccessMana] = objectstorage.New[*mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixAccess))
	storages[mana.ConsensusMana] = objectstorage.New[*mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixConsensus))
	if ManaParameters.EnableResearchVectors {
		storages[mana.ResearchAccess] = objectstorage.New[*mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixAccessResearch))
		storages[mana.ResearchConsensus] = objectstorage.New[*mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixConsensusResearch))
	}
	// consensusEventsLogStorage = osFactory.New(mana.PrefixEventStorage, mana.FromEventObjectStorage)
	// consensusEventsLogsStorageSize.Store(getConsensusEventLogsStorageSize())
	// manaLogger.Infof("read %d mana events from storage", consensusEventsLogsStorageSize.Load())
	// consensusBaseManaPastVectorStorage = osFactory.New(mana.PrefixConsensusPastVector, mana.FromObjectStorage)
	// consensusBaseManaPastVectorMetadataStorage = osFactory.New(mana.PrefixConsensusPastMetadata, mana.FromMetadataObjectStorage)

	err := verifyPledgeNodes()
	if err != nil {
		manaLogger.Panic(err.Error())
	}

	// debuggingEnabled = ManaParameters.DebuggingEnabled

	configureEvents()
}

func configureEvents() {
	// until we have the proper event...
	deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Attach(onTransactionConfirmedClosure)
	// mana.Events().Revoked.Hook(onRevokeEventClosure)
}

//func logPledgeEvent(ev *mana.PledgedEvent) {
//	if ev.ManaType == mana.ConsensusMana {
//		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
//		consensusEventsLogsStorageSize.Inc()
//	}
//}
//
//func logRevokeEvent(ev *mana.RevokedEvent) {
//	if ev.ManaType == mana.ConsensusMana {
//		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
//		consensusEventsLogsStorageSize.Inc()
//	}
//}

func onTransactionConfirmed(transactionID ledgerstate.TransactionID) {
	deps.Tangle.LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		// holds all info mana pkg needs for correct mana calculations from the transaction
		var txInfo *mana.TxInfo

		// process transaction object to build txInfo
		totalAmount, inputInfos := gatherInputInfos(transaction)

		txInfo = &mana.TxInfo{
			TimeStamp:     transaction.Essence().Timestamp(),
			TransactionID: transactionID,
			TotalBalance:  totalAmount,
			PledgeID: map[mana.Type]identity.ID{
				mana.AccessMana:    transaction.Essence().AccessPledgeID(),
				mana.ConsensusMana: transaction.Essence().ConsensusPledgeID(),
			},
			InputInfos: inputInfos,
		}

		// book in all mana vectors.
		for _, baseManaVector := range baseManaVectors {
			baseManaVector.Book(txInfo)
		}
	})
}

func gatherInputInfos(transaction *ledgerstate.Transaction) (totalAmount float64, inputInfos []mana.InputInfo) {
	inputInfos = make([]mana.InputInfo, 0)
	for _, input := range transaction.Essence().Inputs() {
		var inputInfo mana.InputInfo

		deps.Tangle.LedgerState.CachedOutput(input.(*ledgerstate.UTXOInput).ReferencedOutputID()).Consume(func(o ledgerstate.Output) {
			inputInfo.InputID = o.ID()

			// first, sum balances of the input, calculate total amount as well for later
			o.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				inputInfo.Amount += float64(balance)
				totalAmount += float64(balance)
				return true
			})

			// derive the transaction that created this input
			inputTxID := o.ID().TransactionID()
			// look into the transaction, we need timestamp and access & consensus pledge IDs
			deps.Tangle.LedgerState.Transaction(inputTxID).Consume(func(transaction *ledgerstate.Transaction) {
				if transaction == nil {
					return
				}
				inputInfo.TimeStamp = transaction.Essence().Timestamp()
				inputInfo.PledgeID = map[mana.Type]identity.ID{
					mana.AccessMana:    transaction.Essence().AccessPledgeID(),
					mana.ConsensusMana: transaction.Essence().ConsensusPledgeID(),
				}
			})
		})
		inputInfos = append(inputInfos, inputInfo)
	}
	return totalAmount, inputInfos
}

func runManaPlugin(_ *node.Plugin) {
	// mana calculation coefficients can be set from config
	ema1 := ManaParameters.EmaCoefficient1
	ema2 := ManaParameters.EmaCoefficient2
	dec := ManaParameters.Decay
	pruneInterval := ManaParameters.PruneConsensusEventLogsInterval
	vectorsCleanUpInterval := ManaParameters.VectorsCleanupInterval
	fmt.Printf("Prune interval: %v\n", pruneInterval)
	mana.SetCoefficients(ema1, ema2, dec)
	if err := daemon.BackgroundWorker("Mana", func(ctx context.Context) {
		defer manaLogger.Infof("Stopping %s ... done", PluginName)
		// ticker := time.NewTicker(pruneInterval)
		// defer ticker.Stop()
		cleanupTicker := time.NewTicker(vectorsCleanUpInterval)
		defer cleanupTicker.Stop()
		if !readStoredManaVectors() {
			// read snapshot file
			if Parameters.Snapshot.File != "" {
				snapshot := &ledgerstate.Snapshot{}
				f, err := os.Open(Parameters.Snapshot.File)
				if err != nil {
					Plugin.Panic("can not open snapshot file:", err)
				}
				if _, err := snapshot.ReadFrom(f); err != nil {
					Plugin.Panic("could not read snapshot file in Mana Plugin:", err)
				}
				loadSnapshot(snapshot)

				// initialize cMana WeightProvider with snapshot
				t := time.Unix(tangle.DefaultGenesisTime, 0)
				genesisNodeID := identity.ID{}
				for nodeID := range GetCMana() {
					if nodeID == genesisNodeID {
						continue
					}
					deps.Tangle.WeightProvider.Update(t, nodeID)
				}

				manaLogger.Infof("MANA: read snapshot from %s", Parameters.Snapshot.File)
			}
		}
		pruneStorages()
		for {
			select {
			case <-ctx.Done():
				manaLogger.Infof("Stopping %s ...", PluginName)
				// mana.Events().Pledged.Detach(onPledgeEventClosure)
				// mana.Events().Pledged.Detach(onRevokeEventClosure)
				deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Detach(onTransactionConfirmedClosure)
				storeManaVectors()
				shutdownStorages()
				return
			// case <-ticker.C:
			// pruneConsensusEventLogsStorage()
			case <-cleanupTicker.C:
				cleanupManaVectors()
			}
		}
	}, shutdown.PriorityMana); err != nil {
		manaLogger.Panicf("Failed to start as daemon: %s", err)
	}
}

func readStoredManaVectors() (read bool) {
	for vectorType := range baseManaVectors {
		storages[vectorType].ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*mana.PersistableBaseMana]) bool {
			cachedObject.Consume(func(p *mana.PersistableBaseMana) {
				err := baseManaVectors[vectorType].FromPersistable(p)
				if err != nil {
					manaLogger.Errorf("error while restoring %s mana vector: %s", vectorType.String(), err.Error())
				}
				read = true
			})
			return true
		})
	}
	return
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
	// consensusEventsLogStorage.Shutdown()
	// consensusBaseManaPastVectorStorage.Shutdown()
	// consensusBaseManaPastVectorMetadataStorage.Shutdown()
}

// GetHighestManaNodes returns the n highest type mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func GetHighestManaNodes(manaType mana.Type, n uint) ([]mana.Node, time.Time, error) {
	if !QueryAllowed() {
		return []mana.Node{}, time.Now(), ErrQueryNotAllowed
	}
	bmv := baseManaVectors[manaType]
	return bmv.GetHighestManaNodes(n)
}

// GetHighestManaNodesFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each node.
// If p is zero or greater than one, it returns all nodes.
func GetHighestManaNodesFraction(manaType mana.Type, p float64) ([]mana.Node, time.Time, error) {
	if !QueryAllowed() {
		return []mana.Node{}, time.Now(), ErrQueryNotAllowed
	}
	bmv := baseManaVectors[manaType]
	return bmv.GetHighestManaNodesFraction(p)
}

// GetManaMap returns type mana perception of the node.
func GetManaMap(manaType mana.Type, optionalUpdateTime ...time.Time) (mana.NodeMap, time.Time, error) {
	if !QueryAllowed() {
		return mana.NodeMap{}, time.Now(), ErrQueryNotAllowed
	}
	return baseManaVectors[manaType].GetManaMap(optionalUpdateTime...)
}

// GetCMana is a wrapper for the approval weight.
func GetCMana() map[identity.ID]float64 {
	m, _, err := GetManaMap(mana.ConsensusMana)
	if err != nil {
		panic(err)
	}
	return m
}

// GetTotalMana returns sum of mana of all nodes in the network.
func GetTotalMana(manaType mana.Type, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	if !QueryAllowed() {
		return 0, time.Now(), ErrQueryNotAllowed
	}
	manaMap, updateTime, err := baseManaVectors[manaType].GetManaMap(optionalUpdateTime...)
	if err != nil {
		return 0, time.Now(), err
	}

	var sum float64
	for _, m := range manaMap {
		sum += m
	}
	return sum, updateTime, nil
}

// GetAccessMana returns the access mana of the node specified.
func GetAccessMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	if !QueryAllowed() {
		return 0, time.Now(), ErrQueryNotAllowed
	}
	return baseManaVectors[mana.AccessMana].GetMana(nodeID, optionalUpdateTime...)
}

// GetConsensusMana returns the consensus mana of the node specified.
func GetConsensusMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	if !QueryAllowed() {
		return 0, time.Now(), ErrQueryNotAllowed
	}
	return baseManaVectors[mana.ConsensusMana].GetMana(nodeID, optionalUpdateTime...)
}

// GetNeighborsMana returns the type mana of the nodes neighbors.
func GetNeighborsMana(manaType mana.Type, neighbors []*gossip.Neighbor, optionalUpdateTime ...time.Time) (mana.NodeMap, error) {
	if !QueryAllowed() {
		return mana.NodeMap{}, ErrQueryNotAllowed
	}

	res := make(mana.NodeMap)
	for _, n := range neighbors {
		// in case of error, value is 0.0
		value, _, _ := baseManaVectors[manaType].GetMana(n.ID(), optionalUpdateTime...)
		res[n.ID()] = value
	}
	return res, nil
}

// GetAllManaMaps returns the full mana maps for comparison with the perception of other nodes.
func GetAllManaMaps(optionalUpdateTime ...time.Time) (map[mana.Type]mana.NodeMap, error) {
	if !QueryAllowed() {
		return make(map[mana.Type]mana.NodeMap), ErrQueryNotAllowed
	}
	res := make(map[mana.Type]mana.NodeMap)
	for manaType := range baseManaVectors {
		res[manaType], _, _ = GetManaMap(manaType, optionalUpdateTime...)
	}
	return res, nil
}

// OverrideMana sets the nodes mana to a specific value.
// It can be useful for debugging, setting faucet mana, initialization, etc.. Triggers ManaUpdated.
func OverrideMana(manaType mana.Type, nodeID identity.ID, bm *mana.AccessBaseMana) {
	baseManaVectors[manaType].SetMana(nodeID, bm)
}

// GetAllowedPledgeNodes returns the list of nodes that type mana is allowed to be pledged to.
func GetAllowedPledgeNodes(manaType mana.Type) AllowedPledge {
	return allowedPledgeNodes[manaType]
}

// GetOnlineNodes gets the list of currently known (and verified) peers in the network, and their respective mana values.
// Sorted in descending order based on mana. Zero mana nodes are excluded.
func GetOnlineNodes(manaType mana.Type) (onlineNodesMana []mana.Node, t time.Time, err error) {
	if !QueryAllowed() {
		return []mana.Node{}, time.Now(), ErrQueryNotAllowed
	}
	if deps.Discover == nil {
		return
	}
	knownPeers := deps.Discover.GetVerifiedPeers()
	// consider ourselves as a peer in the network too
	knownPeers = append(knownPeers, deps.Local.Peer)
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
		IsFilterEnabled: ManaParameters.AllowedAccessFilterEnabled,
	}
	consensus := AllowedPledge{
		IsFilterEnabled: ManaParameters.AllowedConsensusFilterEnabled,
	}

	access.Allowed = set.New[identity.ID](false)
	// own ID is allowed by default
	access.Allowed.Add(deps.Local.ID())
	if access.IsFilterEnabled {
		for _, pubKey := range ManaParameters.AllowedAccessPledge {
			ID, err := mana.IDFromStr(pubKey)
			if err != nil {
				return err
			}
			access.Allowed.Add(ID)
		}
	}

	consensus.Allowed = set.New[identity.ID](false)
	// own ID is allowed by default
	consensus.Allowed.Add(deps.Local.ID())
	if consensus.IsFilterEnabled {
		for _, pubKey := range ManaParameters.AllowedConsensusPledge {
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
	cachedOutputMetadata := deps.Tangle.LedgerState.CachedOutputMetadata(outputID)
	defer cachedOutputMetadata.Release()
	outputMetadata, exists := cachedOutputMetadata.Unwrap()

	// spent output has 0 pending mana.
	if !exists || outputMetadata.ConsumerCount() > 0 {
		return 0, time.Time{}
	}

	var value float64
	deps.Tangle.LedgerState.CachedOutput(outputID).Consume(func(output ledgerstate.Output) {
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			value += float64(balance)
			return true
		})
	})

	cachedTx := deps.Tangle.LedgerState.Transaction(outputID.TransactionID())
	defer cachedTx.Release()
	tx, _ := cachedTx.Unwrap()
	txTimestamp := tx.Essence().Timestamp()
	return GetPendingMana(value, time.Since(txTimestamp)), txTimestamp
}

// GetPendingMana returns the mana pledged by spending a `value` output that sat for `n` duration.
func GetPendingMana(value float64, n time.Duration) float64 {
	return value * (1 - math.Pow(math.E, -mana.Decay*(n.Seconds())))
}

//// GetLoggedEvents gets the events logs for the node IDs and time frame specified. If none is specified, it returns the logs for all nodes.
//func GetLoggedEvents(identityIDs []identity.ID, startTime time.Time, endTime time.Time) (map[identity.ID]*EventsLogs, error) {
//	logs := make(map[identity.ID]*EventsLogs)
//	lookup := make(map[identity.ID]bool)
//	getAll := true
//
//	if len(identityIDs) > 0 {
//		getAll = false
//		for _, nodeID := range identityIDs {
//			lookup[nodeID] = true
//		}
//	}
//
//	var err error
//	consensusEventsLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
//		cachedPe := &mana.CachedPersistableEvent{CachedObject: cachedObject}
//		defer cachedPe.Release()
//		pbm := cachedPe.Unwrap()
//
//		if !getAll {
//			if !lookup[pbm.NodeID] {
//				return true
//			}
//		}
//
//		if _, found := logs[pbm.NodeID]; !found {
//			logs[pbm.NodeID] = &EventsLogs{}
//		}
//
//		var ev mana.Event
//		ev, err = mana.FromPersistableEvent(pbm)
//		if err != nil {
//			return false
//		}
//
//		if ev.Timestamp().Before(startTime) || ev.Timestamp().After(endTime) {
//			return true
//		}
//		switch ev.Type() {
//		case mana.EventTypePledge:
//			logs[pbm.NodeID].Pledge = append(logs[pbm.NodeID].Pledge, ev.(*mana.PledgedEvent))
//		case mana.EventTypeRevoke:
//			logs[pbm.NodeID].Revoke = append(logs[pbm.NodeID].Revoke, ev.(*mana.RevokedEvent))
//		default:
//			err = mana.ErrUnknownManaEvent
//			return false
//		}
//		return true
//	})
//
//	for ID := range logs {
//		sort.Slice(logs[ID].Pledge, func(i, j int) bool {
//			return logs[ID].Pledge[i].Time.Before(logs[ID].Pledge[j].Time)
//		})
//		sort.Slice(logs[ID].Revoke, func(i, j int) bool {
//			return logs[ID].Revoke[i].Time.Before(logs[ID].Revoke[j].Time)
//		})
//	}
//
//	return logs, err
//}
//
//// GetPastConsensusManaVectorMetadata gets the past consensus mana vector metadata.
//func GetPastConsensusManaVectorMetadata() *mana.ConsensusBasePastManaVectorMetadata {
//	cachedObj := consensusBaseManaPastVectorMetadataStorage.Load([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
//	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
//	defer cachedMetadata.Release()
//	return cachedMetadata.Unwrap()
//}
//
//// GetPastConsensusManaVector builds a consensus base mana vector in the past.
//func GetPastConsensusManaVector(t time.Time) (*mana.ConsensusBaseManaVector, []mana.Event, error) {
//	baseManaVector, err := mana.NewBaseManaVector(mana.ConsensusMana)
//	if err != nil {
//		return nil, nil, err
//	}
//	cbmvPast := baseManaVector.(*mana.ConsensusBaseManaVector)
//	cachedObj := consensusBaseManaPastVectorMetadataStorage.Load([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
//	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
//	defer cachedMetadata.Release()
//
//	if cachedMetadata.Exists() {
//		metadata := cachedMetadata.Unwrap()
//		if t.After(metadata.Timestamp) {
//			consensusBaseManaPastVectorStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
//				cachedPbm := &mana.CachedPersistableBaseMana{CachedObject: cachedObject}
//				defer cachedPbm.Release()
//				p := cachedPbm.Unwrap()
//				err = cbmvPast.FromPersistable(p)
//				if err != nil {
//					manaLogger.Errorf("error while restoring %s mana vector from storage: %w", mana.ConsensusMana.String(), err)
//					baseManaVector, _ := mana.NewBaseManaVector(mana.ConsensusMana)
//					cbmvPast = baseManaVector.(*mana.ConsensusBaseManaVector)
//					return false
//				}
//				return true
//			})
//		}
//	}
//
//	var eventLogs mana.EventSlice
//	consensusEventsLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
//		cachedPe := &mana.CachedPersistableEvent{CachedObject: cachedObject}
//		defer cachedPe.Release()
//		pe := cachedPe.Unwrap()
//		if pe.Time.After(t) {
//			return true
//		}
//
//		// already consumed in stored base mana vector.
//		if cachedMetadata.Exists() && cbmvPast.Size() > 0 {
//			metadata := cachedMetadata.Unwrap()
//			if pe.Time.Before(metadata.Timestamp) {
//				return true
//			}
//		}
//
//		var ev mana.Event
//		ev, err = mana.FromPersistableEvent(pe)
//		if err != nil {
//			return false
//		}
//		eventLogs = append(eventLogs, ev)
//		return true
//	})
//	if err != nil {
//		return nil, nil, err
//	}
//	eventLogs.Sort()
//	err = cbmvPast.BuildPastBaseVector(eventLogs, t)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	err = cbmvPast.UpdateAll(t)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	return cbmvPast, eventLogs, nil
//}
//
//func getConsensusEventLogsStorageSize() uint32 {
//	var size uint32
//	consensusEventsLogStorage.ForEachKeyOnly(func(key []byte) bool {
//		size++
//		return true
//	}, objectstorage.WithIteratorSkipCache(true))
//	return size
//}
//
//func pruneConsensusEventLogsStorage() {
//	if consensusEventsLogsStorageSize.Load() < maxConsensusEventsInStorage {
//		return
//	}
//
//	cachedObj := consensusBaseManaPastVectorMetadataStorage.Load([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
//	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
//	defer cachedMetadata.Release()
//
//	bmv, err := mana.NewBaseManaVector(mana.ConsensusMana)
//	if err != nil {
//		manaLogger.Errorf("error creating consensus base mana vector: %v", err)
//		return
//	}
//	cbmvPast := bmv.(*mana.ConsensusBaseManaVector)
//	if cachedMetadata.Exists() {
//		consensusBaseManaPastVectorStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
//			cachedPbm := &mana.CachedPersistableBaseMana{CachedObject: cachedObject}
//			pbm := cachedPbm.Unwrap()
//			if pbm != nil {
//				err = cbmvPast.FromPersistable(pbm)
//				if err != nil {
//					return false
//				}
//			}
//			return true
//		})
//		if err != nil {
//			manaLogger.Errorf("error reading stored consensus base mana vector: %v", err)
//			return
//		}
//	}
//
//	var eventLogs mana.EventSlice
//	consensusEventsLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
//		cachedPe := &mana.CachedPersistableEvent{CachedObject: cachedObject}
//		defer cachedPe.Release()
//		pe := cachedPe.Unwrap()
//		var ev mana.Event
//		ev, err = mana.FromPersistableEvent(pe)
//
//		if cachedMetadata.Exists() {
//			metadata := cachedMetadata.Unwrap()
//			if ev.Timestamp().Before(metadata.Timestamp) {
//				manaLogger.Errorf("consensus event storage contains event that is older, than the stored metadata timestamp %s: %s", metadata.Timestamp, ev.String())
//				return true
//			}
//		}
//
//		if err != nil {
//			return false
//		}
//		eventLogs = append(eventLogs, ev)
//		return true
//	})
//	if err != nil {
//		manaLogger.Infof("error reading persistable events: %v", err)
//		return
//	}
//	eventLogs.Sort()
//	// we always want (maxConsensusEventsInStorage - slidingEventsInterval) number of events left
//	deleteWindow := len(eventLogs) - (maxConsensusEventsInStorage - slidingEventsInterval)
//	storageSizeInt := int(consensusEventsLogsStorageSize.Load())
//	if deleteWindow < 0 || deleteWindow > storageSizeInt {
//		manaLogger.Errorf("invalid delete window %d for storage size %d, max storage size %d and sliding interval %d",
//			deleteWindow, storageSizeInt, maxConsensusEventsInStorage, slidingEventsInterval)
//		return
//	}
//	// Make sure to take related events. (we take deleteWindow oldest events)
//	// Ensures that related events (same time) are not split between different intervals.
//	prev := eventLogs[deleteWindow-1]
//	var i int
//	for i = deleteWindow; i < len(eventLogs); i++ {
//		if !eventLogs[i].Timestamp().Equal(prev.Timestamp()) {
//			break
//		}
//		prev = eventLogs[i]
//	}
//	toBePrunedEvents := eventLogs[:i]
//	// TODO: later, when we have epochs, we have to make sure that `t` is before the epoch to be "finalized" next.
//	// Otherwise, we won't be able to calculate the consensus mana for that epoch because we already pruned the events
//	// leading up to it.
//	t := toBePrunedEvents[len(toBePrunedEvents)-1].Timestamp()
//
//	err = cbmvPast.BuildPastBaseVector(toBePrunedEvents, t)
//	if err != nil {
//		manaLogger.Errorf("error building past consensus base mana vector: %w", err)
//		return
//	}
//
//	// store cbmv
//	if err = consensusBaseManaPastVectorStorage.Prune(); err != nil {
//		manaLogger.Errorf("error pruning consensus base mana vector storage: %w", err)
//		return
//	}
//	for _, p := range cbmvPast.ToPersistables() {
//		consensusBaseManaPastVectorStorage.Store(p).Release()
//	}
//
//	// store the metadata
//	metadata := &mana.ConsensusBasePastManaVectorMetadata{
//		Timestamp: t,
//	}
//
//	if err = consensusBaseManaPastVectorMetadataStorage.Prune(); err != nil {
//		manaLogger.Errorf("error pruning consensus base mana vector metadata storage: %w", err)
//		return
//	}
//	consensusBaseManaPastVectorMetadataStorage.Store(metadata).Release()
//
//	var entriesToDelete [][]byte
//	for _, ev := range toBePrunedEvents {
//		entriesToDelete = append(entriesToDelete, ev.ToPersistable().ObjectStorageKey())
//	}
//	manaLogger.Infof("deleting %d events from consensus event storage", len(entriesToDelete))
//	consensusEventsLogStorage.DeleteEntriesFromStore(entriesToDelete)
//	consensusEventsLogsStorageSize.Sub(uint32(len(entriesToDelete)))
//	manaLogger.Infof("%d events remaining in consensus event storage", consensusEventsLogsStorageSize.Load())
//}

func cleanupManaVectors() {
	vectorTypes := []mana.Type{mana.AccessMana, mana.ConsensusMana}
	if ManaParameters.EnableResearchVectors {
		vectorTypes = append(vectorTypes, mana.ResearchAccess)
		vectorTypes = append(vectorTypes, mana.ResearchConsensus)
	}
	for _, vecType := range vectorTypes {
		baseManaVectors[vecType].RemoveZeroNodes()
	}
}

// AllowedPledge represents the nodes that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool
	Allowed         set.Set[identity.ID]
}

//// EventsLogs represents the events logs.
//type EventsLogs struct {
//	Pledge []*mana.PledgedEvent `json:"pledge"`
//	Revoke []*mana.RevokedEvent `json:"revoke"`
//}

// QueryAllowed returns if the mana plugin answers queries or not.
func QueryAllowed() (allowed bool) {
	// if debugging enabled, reply to the query
	// if debugging is not allowed, only reply when in sync
	// return deps.Tangle.Synced() || debuggingEnabled\

	// query allowed only when base mana vectors have been initialized
	return len(baseManaVectors) > 0
}

// loadSnapshot loads the tx snapshot and the access mana snapshot, sorts it and loads it into the various mana versions.
func loadSnapshot(snapshot *ledgerstate.Snapshot) {
	txSnapshotByNode := make(map[identity.ID]mana.SortedTxSnapshot)

	// load txSnapshot into SnapshotInfoVec
	for txID, record := range snapshot.Transactions {
		totalUnspentBalanceInTx := uint64(0)
		for i, output := range record.Essence.Outputs() {
			if !record.UnspentOutputs[i] {
				continue
			}
			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				totalUnspentBalanceInTx += balance
				return true
			})
		}
		txInfo := &mana.TxSnapshot{
			Value:     float64(totalUnspentBalanceInTx),
			TxID:      txID,
			Timestamp: record.Essence.Timestamp(),
		}
		txSnapshotByNode[record.Essence.ConsensusPledgeID()] = append(txSnapshotByNode[record.Essence.ConsensusPledgeID()], txInfo)
	}

	// sort txSnapshot per nodeID, so that for each nodeID it is in temporal order
	snapshotByNode := make(map[identity.ID]mana.SnapshotNode)
	for nodeID := range txSnapshotByNode {
		sort.Sort(txSnapshotByNode[nodeID])
		snapshotNode := mana.SnapshotNode{
			SortedTxSnapshot: txSnapshotByNode[nodeID],
		}
		snapshotByNode[nodeID] = snapshotNode
	}

	// determine addTime if snapshot should be updated for the difference to now
	var addTime time.Duration
	// for certain applications (e.g. docker-network) update all timestamps, to have large enough aMana
	maxTimestamp := time.Unix(tangle.DefaultGenesisTime, 0)
	if ManaParameters.SnapshotResetTime {
		for _, accessMana := range snapshot.AccessManaByNode {
			if accessMana.Timestamp.After(maxTimestamp) {
				maxTimestamp = accessMana.Timestamp
			}
		}
		addTime = time.Since(maxTimestamp)
	}

	// load access mana
	for nodeID, accessMana := range snapshot.AccessManaByNode {
		snapshotNode, ok := snapshotByNode[nodeID]
		if !ok { // fill with empty element if it does not exist yet
			snapshotNode = mana.SnapshotNode{}
			snapshotByNode[nodeID] = snapshotNode
		}
		snapshotNode.AccessMana = mana.AccessManaSnapshot{
			Value:     accessMana.Value,
			Timestamp: accessMana.Timestamp.Add(addTime),
		}
		snapshotByNode[nodeID] = snapshotNode
	}

	baseManaVectors[mana.ConsensusMana].LoadSnapshot(snapshotByNode)
	baseManaVectors[mana.AccessMana].LoadSnapshot(snapshotByNode)
}
