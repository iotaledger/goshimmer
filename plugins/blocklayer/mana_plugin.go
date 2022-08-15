package blocklayer

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	db_pkg "github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/mana"

	"github.com/iotaledger/goshimmer/packages/core/snapshot"
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
	onTransactionAcceptedClosure *event.Closure[*ledger.TransactionAcceptedEvent]
	onManaVectorToUpdateClosure  *event.Closure[*notarization.ManaVectorUpdateEvent]
	// onPledgeEventClosure          *events.Closure
	// onRevokeEventClosure          *events.Closure
	// debuggingEnabled              bool.
)

func init() {
	ManaPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(func() mana.ManaRetrievalFunc {
			return GetConsensusMana
		}, dig.Name("manaFunc")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configureManaPlugin(*node.Plugin) {
	manaLogger = logger.NewLogger(PluginName)

	onTransactionAcceptedClosure = event.NewClosure(func(event *ledger.TransactionAcceptedEvent) { onTransactionAccepted(event.TransactionID) })
	onManaVectorToUpdateClosure = event.NewClosure(func(event *notarization.ManaVectorUpdateEvent) {
		baseManaVectors[mana.ConsensusMana].BookEpoch(event.EpochDiffCreated, event.EpochDiffSpent)
	})
	// onPledgeEventClosure = events.NewClosure(logPledgeEvent)
	// onRevokeEventClosure = events.NewClosure(logRevokeEvent)

	allowedPledgeNodes = make(map[mana.Type]AllowedPledge)
	baseManaVectors = make(map[mana.Type]mana.BaseManaVector)
	baseManaVectors[mana.AccessMana] = mana.NewBaseManaVector(mana.AccessMana)
	baseManaVectors[mana.ConsensusMana] = mana.NewBaseManaVector(mana.ConsensusMana)

	// configure storage for each vector type
	storages = make(map[mana.Type]*objectstorage.ObjectStorage[*mana.PersistableBaseMana])
	store := deps.Storage
	storages[mana.AccessMana] = objectstorage.NewStructStorage[mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixAccess))
	storages[mana.ConsensusMana] = objectstorage.NewStructStorage[mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixConsensus))
	if ManaParameters.EnableResearchVectors {
		storages[mana.ResearchAccess] = objectstorage.NewStructStorage[mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixAccessResearch))
		storages[mana.ResearchConsensus] = objectstorage.NewStructStorage[mana.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, db_pkg.PrefixMana, mana.PrefixConsensusResearch))
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
	deps.Tangle.Ledger.Events.TransactionAccepted.Attach(onTransactionAcceptedClosure)
	// mana.Events().Revoked.Attach(onRevokeEventClosure)
}

// func logPledgeEvent(ev *mana.PledgedEvent) {
//	if ev.ManaType == mana.ConsensusMana {
//		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
//		consensusEventsLogsStorageSize.Inc()
//	}
// }
//
// func logRevokeEvent(ev *mana.RevokedEvent) {
//	if ev.ManaType == mana.ConsensusMana {
//		consensusEventsLogStorage.Store(ev.ToPersistable()).Release()
//		consensusEventsLogsStorageSize.Inc()
//	}
// }

func onTransactionAccepted(transactionID utxo.TransactionID) {
	deps.Tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
		// holds all info mana pkg needs for correct mana calculations from the transaction
		var txInfo *mana.TxInfo

		devnetTransaction := transaction.(*devnetvm.Transaction)

		// process transaction object to build txInfo
		totalAmount, inputInfos := gatherInputInfos(devnetTransaction.Essence().Inputs())

		txInfo = &mana.TxInfo{
			TimeStamp:     devnetTransaction.Essence().Timestamp(),
			TransactionID: transactionID,
			TotalBalance:  totalAmount,
			PledgeID: map[mana.Type]identity.ID{
				mana.AccessMana:    devnetTransaction.Essence().AccessPledgeID(),
				mana.ConsensusMana: devnetTransaction.Essence().ConsensusPledgeID(),
			},
			InputInfos: inputInfos,
		}

		// book in only access mana
		baseManaVectors[mana.AccessMana].Book(txInfo)
	})
}

func gatherInputInfos(inputs devnetvm.Inputs) (totalAmount float64, inputInfos []mana.InputInfo) {
	inputInfos = make([]mana.InputInfo, 0)
	for _, input := range inputs {
		var inputInfo mana.InputInfo

		outputID := input.(*devnetvm.UTXOInput).ReferencedOutputID()
		deps.Tangle.Ledger.Storage.CachedOutput(outputID).Consume(func(o utxo.Output) {
			inputInfo.InputID = o.ID()

			// first, sum balances of the input, calculate total amount as well for later
			if amount, exists := o.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
				inputInfo.Amount = float64(amount)
				totalAmount += float64(amount)
			}

			// look into the transaction, we need timestamp and access & consensus pledge IDs
			deps.Tangle.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
				inputInfo.PledgeID = map[mana.Type]identity.ID{
					mana.AccessMana:    metadata.AccessManaPledgeID(),
					mana.ConsensusMana: metadata.ConsensusManaPledgeID(),
				}
			})
		})
		inputInfos = append(inputInfos, inputInfo)
	}
	return totalAmount, inputInfos
}

func runManaPlugin(_ *node.Plugin) {
	// mana calculation coefficients can be set from config
	pruneInterval := ManaParameters.PruneConsensusEventLogsInterval
	vectorsCleanUpInterval := ManaParameters.VectorsCleanupInterval
	fmt.Printf("Prune interval: %v\n", pruneInterval)
	if err := daemon.BackgroundWorker("Mana", func(ctx context.Context) {
		defer manaLogger.Infof("Stopping %s ... done", PluginName)
		// ticker := time.NewTicker(pruneInterval)
		// defer ticker.Stop()
		cleanupTicker := time.NewTicker(vectorsCleanUpInterval)
		defer cleanupTicker.Stop()
		if !readStoredManaVectors() {
			// read snapshot file
			if Parameters.Snapshot.File != "" {
				var cManaTargetEpoch epoch.Index
				consensusManaByNode := map[identity.ID]float64{}
				accessManaByNode := map[identity.ID]float64{}
				processOutputs := func(outputsWithMetadata []*ledger.OutputWithMetadata, baseVector map[identity.ID]float64, areCreated bool) {
					for _, outputWithMetadata := range outputsWithMetadata {
						devnetOutput := outputWithMetadata.Output().(devnetvm.Output)
						balance, exists := devnetOutput.Balances().Get(devnetvm.ColorIOTA)
						if !exists {
							continue
						}
						consensusManaPledgeID := outputWithMetadata.ConsensusManaPledgeID()
						if areCreated {
							baseVector[consensusManaPledgeID] += float64(balance)
						} else {
							baseVector[consensusManaPledgeID] -= float64(balance)
						}
					}

					return
				}

				utxoStatesConsumer := func(outputsWithMetadatas []*ledger.OutputWithMetadata) {
					processOutputs(outputsWithMetadatas, consensusManaByNode, true /* areCreated */)
					processOutputs(outputsWithMetadatas, accessManaByNode, true /* areCreated */)

				}

				epochDiffsConsumer := func(diff *ledger.EpochDiff) {
					// We fix the cMana vector a few epochs in the past with respect of the latest epoch in the snapshot.

					processOutputs(diff.Created(), consensusManaByNode, true /* areCreated */)
					processOutputs(diff.Created(), accessManaByNode, true /* areCreated */)
					processOutputs(diff.Spent(), consensusManaByNode, false /* areCreated */)
					processOutputs(diff.Spent(), accessManaByNode, false /* areCreated */)

					// Only the aMana will be loaded until the latest snapshot's epoch

					processOutputs(diff.Created(), accessManaByNode, true /* areCreated */)
					processOutputs(diff.Spent(), accessManaByNode, false /* areCreated */)

				}

				headerConsumer := func(header *ledger.SnapshotHeader) {
					cManaTargetEpoch = header.DiffEpochIndex - epoch.Index(ManaParameters.EpochDelay)
					if cManaTargetEpoch < 0 {
						cManaTargetEpoch = 0
					}

				}
				emptySepsConsumer := func(*snapshot.SolidEntryPoints) {}

				if err := snapshot.LoadSnapshot(Parameters.Snapshot.File, headerConsumer, emptySepsConsumer, utxoStatesConsumer, epochDiffsConsumer); err != nil {
					Plugin.Panic("could not load snapshot from file", Parameters.Snapshot.File, err)
				}
				baseManaVectors[mana.ConsensusMana].InitializeWithData(consensusManaByNode)
				baseManaVectors[mana.AccessMana].InitializeWithData(accessManaByNode)

				// initialize cMana WeightProvider with snapshot
				// TODO: consume the activity record from the snapshot to determine which nodes were active at the time of the snapshot
				t := deps.Tangle.Options.GenesisTime
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
				deps.Tangle.Ledger.Events.TransactionAccepted.Detach(onTransactionAcceptedClosure)
				deps.NotarizationMgr.Events.ManaVectorUpdate.Detach(onManaVectorToUpdateClosure)
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
	return baseManaVectors[manaType].GetManaMap()
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
	manaMap, updateTime, err := baseManaVectors[manaType].GetManaMap()
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
	return baseManaVectors[mana.AccessMana].GetMana(nodeID)
}

// GetConsensusMana returns the consensus mana of the node specified.
func GetConsensusMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	if !QueryAllowed() {
		return 0, time.Now(), ErrQueryNotAllowed
	}
	return baseManaVectors[mana.ConsensusMana].GetMana(nodeID)
}

// GetNeighborsMana returns the type mana of the nodes neighbors.
func GetNeighborsMana(manaType mana.Type, neighbors []*p2p.Neighbor, optionalUpdateTime ...time.Time) (mana.NodeMap, error) {
	if !QueryAllowed() {
		return mana.NodeMap{}, ErrQueryNotAllowed
	}

	res := make(mana.NodeMap)
	for _, n := range neighbors {
		// in case of error, value is 0.0
		value, _, _ := baseManaVectors[manaType].GetMana(n.ID())
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

// // GetLoggedEvents gets the events logs for the node IDs and time frame specified. If none is specified, it returns the logs for all nodes.
// func GetLoggedEvents(identityIDs []identity.ID, startTime time.Time, endTime time.Time) (map[identity.ID]*EventsLogs, error) {
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
// }
//
// // GetPastConsensusManaVectorMetadata gets the past consensus mana vector metadata.
// func GetPastConsensusManaVectorMetadata() *mana.ConsensusBasePastManaVectorMetadata {
//	cachedObj := consensusBaseManaPastVectorMetadataStorage.Load([]byte(mana.ConsensusBaseManaPastVectorMetadataStorageKey))
//	cachedMetadata := &mana.CachedConsensusBasePastManaVectorMetadata{CachedObject: cachedObj}
//	defer cachedMetadata.Release()
//	return cachedMetadata.Unwrap()
// }
//
// // GetPastConsensusManaVector builds a consensus base mana vector in the past.
// func GetPastConsensusManaVector(t time.Time) (*mana.ConsensusBaseManaVector, []mana.Event, error) {
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
// }
//
// func getConsensusEventLogsStorageSize() uint32 {
//	var size uint32
//	consensusEventsLogStorage.ForEachKeyOnly(func(key []byte) bool {
//		size++
//		return true
//	}, objectstorage.WithIteratorSkipCache(true))
//	return size
// }
//
// func pruneConsensusEventLogsStorage() {
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
// }

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

// // EventsLogs represents the events logs.
// type EventsLogs struct {
//	Pledge []*mana.PledgedEvent `json:"pledge"`
//	Revoke []*mana.RevokedEvent `json:"revoke"`
// }

// QueryAllowed returns if the mana plugin answers queries or not.
func QueryAllowed() (allowed bool) {
	// if debugging enabled, reply to the query
	// if debugging is not allowed, only reply when in sync
	// return deps.Tangle.Bootstrapped() || debuggingEnabled\

	// query allowed only when base mana vectors have been initialized
	return len(baseManaVectors) > 0
}
