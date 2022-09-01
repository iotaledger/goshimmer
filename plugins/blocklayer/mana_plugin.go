package blocklayer

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	mana2 "github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/mana"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"

	"github.com/iotaledger/goshimmer/packages/core/snapshot"
)

const (
	// PluginName is the name of the mana plugin.
	PluginName = "Mana"
)

var (
	// ManaPlugin is the plugin instance of the mana plugin.
	ManaPlugin                   = node.NewPlugin(PluginName, nil, node.Enabled, configureManaPlugin, runManaPlugin)
	manaLogger                   *logger.Logger
	baseManaVectors              map[mana2.Type]mana2.BaseManaVector
	storages                     map[mana2.Type]*objectstorage.ObjectStorage[*mana2.PersistableBaseMana]
	allowedPledgeNodes           map[mana2.Type]AllowedPledge
	onTransactionAcceptedClosure *event.Closure[*ledger.TransactionAcceptedEvent]
	onManaVectorToUpdateClosure  *event.Closure[*notarization.ManaVectorUpdateEvent]
)

func init() {
	ManaPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(func() mana2.ManaRetrievalFunc {
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
		manaVectorEI := event.EI - epoch.Index(ManaParameters.EpochDelay)
		if manaVectorEI < 1 {
			return
		}
		spent, created := deps.NotarizationMgr.GetEpochDiff(manaVectorEI)
		baseManaVectors[mana2.ConsensusMana].BookEpoch(created, spent)
	})

	allowedPledgeNodes = make(map[mana2.Type]AllowedPledge)
	baseManaVectors = make(map[mana2.Type]mana2.BaseManaVector)
	baseManaVectors[mana2.AccessMana] = mana2.NewBaseManaVector(mana2.AccessMana)
	baseManaVectors[mana2.ConsensusMana] = mana2.NewBaseManaVector(mana2.ConsensusMana)

	// configure storage for each vector type
	storages = make(map[mana2.Type]*objectstorage.ObjectStorage[*mana2.PersistableBaseMana])
	store := deps.Storage
	storages[mana2.AccessMana] = objectstorage.NewStructStorage[mana2.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, database.PrefixMana, mana2.PrefixAccess))
	storages[mana2.ConsensusMana] = objectstorage.NewStructStorage[mana2.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, database.PrefixMana, mana2.PrefixConsensus))
	if ManaParameters.EnableResearchVectors {
		storages[mana2.ResearchAccess] = objectstorage.NewStructStorage[mana2.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, database.PrefixMana, mana2.PrefixAccessResearch))
		storages[mana2.ResearchConsensus] = objectstorage.NewStructStorage[mana2.PersistableBaseMana](objectstorage.NewStoreWithRealm(store, database.PrefixMana, mana2.PrefixConsensusResearch))
	}

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

func onTransactionAccepted(transactionID utxo.TransactionID) {
	deps.Tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
		// holds all info mana pkg needs for correct mana calculations from the transaction
		var txInfo *mana2.TxInfo

		devnetTransaction := transaction.(*devnetvm.Transaction)

		// process transaction object to build txInfo
		totalAmount, inputInfos := gatherInputInfos(devnetTransaction.Essence().Inputs())

		txInfo = &mana2.TxInfo{
			TimeStamp:     devnetTransaction.Essence().Timestamp(),
			TransactionID: transactionID,
			TotalBalance:  totalAmount,
			PledgeID: map[mana2.Type]identity.ID{
				mana2.AccessMana:    devnetTransaction.Essence().AccessPledgeID(),
				mana2.ConsensusMana: devnetTransaction.Essence().ConsensusPledgeID(),
			},
			InputInfos: inputInfos,
		}

		// book in only access mana
		baseManaVectors[mana2.AccessMana].Book(txInfo)
	})
}

func gatherInputInfos(inputs devnetvm.Inputs) (totalAmount float64, inputInfos []mana2.InputInfo) {
	inputInfos = make([]mana2.InputInfo, 0)
	for _, input := range inputs {
		var inputInfo mana2.InputInfo

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
				inputInfo.PledgeID = map[mana2.Type]identity.ID{
					mana2.AccessMana:    metadata.AccessManaPledgeID(),
					mana2.ConsensusMana: metadata.ConsensusManaPledgeID(),
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

				if err := snapshot.LoadSnapshot(
					Parameters.Snapshot.File,
					headerConsumer,
					emptySepsConsumer,
					utxoStatesConsumer,
					epochDiffsConsumer,
					deps.Tangle.WeightProvider.LoadActiveNodes,
				); err != nil {
					Plugin.Panic("could not load snapshot from file", Parameters.Snapshot.File, err)
				}
				baseManaVectors[mana2.ConsensusMana].InitializeWithData(consensusManaByNode)
				baseManaVectors[mana2.AccessMana].InitializeWithData(accessManaByNode)
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
		storages[vectorType].ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*mana2.PersistableBaseMana]) bool {
			cachedObject.Consume(func(p *mana2.PersistableBaseMana) {
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
func GetHighestManaNodes(manaType mana2.Type, n uint) ([]mana2.Node, time.Time, error) {
	if !QueryAllowed() {
		return []mana2.Node{}, time.Now(), ErrQueryNotAllowed
	}
	bmv := baseManaVectors[manaType]
	return bmv.GetHighestManaNodes(n)
}

// GetHighestManaNodesFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each node.
// If p is zero or greater than one, it returns all nodes.
func GetHighestManaNodesFraction(manaType mana2.Type, p float64) ([]mana2.Node, time.Time, error) {
	if !QueryAllowed() {
		return []mana2.Node{}, time.Now(), ErrQueryNotAllowed
	}
	bmv := baseManaVectors[manaType]
	return bmv.GetHighestManaNodesFraction(p)
}

// GetManaMap returns type mana perception of the node.
func GetManaMap(manaType mana2.Type, optionalUpdateTime ...time.Time) (mana2.NodeMap, time.Time, error) {
	if !QueryAllowed() {
		return mana2.NodeMap{}, time.Now(), ErrQueryNotAllowed
	}
	return baseManaVectors[manaType].GetManaMap()
}

// GetCMana is a wrapper for the approval weight.
func GetCMana() map[identity.ID]float64 {
	m, _, err := GetManaMap(mana2.ConsensusMana)
	if err != nil {
		panic(err)
	}
	return m
}

// GetConfirmedEI is a wrapper for the weightProvider to get confirmed epoch index.
func GetConfirmedEI() epoch.Index {
	ei, err := deps.NotarizationMgr.LatestConfirmedEpochIndex()
	if err != nil {
		panic(err)
	}
	return ei
}

// GetTotalMana returns sum of mana of all nodes in the network.
func GetTotalMana(manaType mana2.Type, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
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
	return baseManaVectors[mana2.AccessMana].GetMana(nodeID)
}

// GetConsensusMana returns the consensus mana of the node specified.
func GetConsensusMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	if !QueryAllowed() {
		return 0, time.Now(), ErrQueryNotAllowed
	}
	return baseManaVectors[mana2.ConsensusMana].GetMana(nodeID)
}

// GetNeighborsMana returns the type mana of the nodes neighbors.
func GetNeighborsMana(manaType mana2.Type, neighbors []*p2p.Neighbor, optionalUpdateTime ...time.Time) (mana2.NodeMap, error) {
	if !QueryAllowed() {
		return mana2.NodeMap{}, ErrQueryNotAllowed
	}

	res := make(mana2.NodeMap)
	for _, n := range neighbors {
		// in case of error, value is 0.0
		value, _, _ := baseManaVectors[manaType].GetMana(n.ID())
		res[n.ID()] = value
	}
	return res, nil
}

// GetAllManaMaps returns the full mana maps for comparison with the perception of other nodes.
func GetAllManaMaps(optionalUpdateTime ...time.Time) (map[mana2.Type]mana2.NodeMap, error) {
	if !QueryAllowed() {
		return make(map[mana2.Type]mana2.NodeMap), ErrQueryNotAllowed
	}
	res := make(map[mana2.Type]mana2.NodeMap)
	for manaType := range baseManaVectors {
		res[manaType], _, _ = GetManaMap(manaType, optionalUpdateTime...)
	}
	return res, nil
}

// GetAllowedPledgeNodes returns the list of nodes that type mana is allowed to be pledged to.
func GetAllowedPledgeNodes(manaType mana2.Type) AllowedPledge {
	return allowedPledgeNodes[manaType]
}

// GetOnlineNodes gets the list of currently known (and verified) peers in the network, and their respective mana values.
// Sorted in descending order based on mana. Zero mana nodes are excluded.
func GetOnlineNodes(manaType mana2.Type) (onlineNodesMana []mana2.Node, t time.Time, err error) {
	if !QueryAllowed() {
		return []mana2.Node{}, time.Now(), ErrQueryNotAllowed
	}
	if deps.Discover == nil {
		return
	}
	knownPeers := deps.Discover.GetVerifiedPeers()
	// consider ourselves as a peer in the network too
	knownPeers = append(knownPeers, deps.Local.Peer)
	onlineNodesMana = make([]mana2.Node, 0)
	for _, peer := range knownPeers {
		if baseManaVectors[manaType].Has(peer.ID()) {
			var peerMana float64
			peerMana, t, err = baseManaVectors[manaType].GetMana(peer.ID())
			if err != nil {
				return nil, t, err
			}
			if peerMana > 0 {
				onlineNodesMana = append(onlineNodesMana, mana2.Node{ID: peer.ID(), Mana: peerMana})
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
			ID, err := mana2.IDFromStr(pubKey)
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
			ID, err := mana2.IDFromStr(pubKey)
			if err != nil {
				return err
			}
			consensus.Allowed.Add(ID)
		}
	}

	allowedPledgeNodes[mana2.AccessMana] = access
	allowedPledgeNodes[mana2.ConsensusMana] = consensus
	return nil
}

func cleanupManaVectors() {
	vectorTypes := []mana2.Type{mana2.AccessMana, mana2.ConsensusMana}
	if ManaParameters.EnableResearchVectors {
		vectorTypes = append(vectorTypes, mana2.ResearchAccess)
		vectorTypes = append(vectorTypes, mana2.ResearchConsensus)
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
