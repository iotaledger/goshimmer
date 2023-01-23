package protocol

import (
	"fmt"
	"sort"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events            *Events
	CongestionControl *congestioncontrol.CongestionControl
	TipManager        *tipmanager.TipManager
	chainManager      *chainmanager.Manager
	engineManager     *enginemanager.EngineManager

	dispatcher      network.Endpoint
	networkProtocol *network.Protocol

	activeEngineMutex sync.RWMutex
	mainEngine        *enginemanager.EngineInstance
	candidateEngine   *enginemanager.EngineInstance

	optsBaseDirectory    string
	optsSnapshotPath     string
	optsPruningThreshold uint64

	// optsSolidificationOptions []options.Option[solidification.Requester]
	optsCongestionControlOptions      []options.Option[congestioncontrol.CongestionControl]
	optsEngineOptions                 []options.Option[engine.Engine]
	optsTipManagerOptions             []options.Option[tipmanager.TipManager]
	optsStorageDatabaseManagerOptions []options.Option[database.Manager]
	optsSybilProtectionProvider       engine.ModuleProvider[sybilprotection.SybilProtection]
	optsThroughputQuotaProvider       engine.ModuleProvider[throughputquota.ThroughputQuota]
}

func New(dispatcher network.Endpoint, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		dispatcher:                  dispatcher,
		optsSybilProtectionProvider: dpos.NewProvider(),
		optsThroughputQuotaProvider: mana1.NewProvider(),

		optsBaseDirectory:    "",
		optsPruningThreshold: 6 * 60, // 1 hour given that epoch duration is 10 seconds
	}, opts,
		(*Protocol).initCongestionControl,
		(*Protocol).initEngineManager,
		(*Protocol).initChainManager,
		(*Protocol).initTipManager,
	)
}

// Run runs the protocol.
func (p *Protocol) Run() {
	p.CongestionControl.Run()
	p.linkTo(p.mainEngine)

	if err := p.mainEngine.InitializeWithSnapshot(p.optsSnapshotPath); err != nil {
		panic(err)
	}

	p.initNetworkProtocol()
}

// Shutdown shuts down the protocol.
func (p *Protocol) Shutdown() {
	p.CongestionControl.Shutdown()

	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	p.mainEngine.Shutdown()

	if p.candidateEngine != nil {
		p.candidateEngine.Shutdown()
	}
}

func (p *Protocol) WorkerPools() map[string]*workerpool.UnboundedWorkerPool {
	wps := make(map[string]*workerpool.UnboundedWorkerPool)
	wps["CongestionControl"] = p.CongestionControl.WorkerPool()
	return lo.MergeMaps(wps, p.MainEngineInstance().Engine.WorkerPools())
}

func (p *Protocol) initEngineManager() {
	p.engineManager = enginemanager.New(
		p.optsBaseDirectory,
		DatabaseVersion,
		p.optsStorageDatabaseManagerOptions,
		p.optsEngineOptions,
		p.optsSybilProtectionProvider,
		p.optsThroughputQuotaProvider,
	)

	p.Events.Engine.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.Engine().Storage.PruneUntilEpoch(epochIndex - epoch.Index(p.optsPruningThreshold))
	}))

	p.mainEngine = lo.PanicOnErr(p.engineManager.LoadActiveEngine())
}

func (p *Protocol) initCongestionControl() {
	p.CongestionControl = congestioncontrol.New(p.optsCongestionControlOptions...)

	p.Events.CongestionControl = p.CongestionControl.Events
}

func (p *Protocol) initNetworkProtocol() {
	p.networkProtocol = network.NewProtocol(p.dispatcher)

	p.networkProtocol.Events.BlockRequestReceived.Attach(event.NewClosure(func(event *network.BlockRequestReceivedEvent) {
		if block, exists := p.MainEngineInstance().Engine.Block(event.BlockID); exists {
			p.networkProtocol.SendBlock(block, event.Source)
		}
	}))

	p.networkProtocol.Events.BlockReceived.Attach(event.NewClosure(func(event *network.BlockReceivedEvent) {
		if err := p.ProcessBlock(event.Block, event.Source); err != nil {
			fmt.Print(err)
		}
	}))

	p.networkProtocol.Events.EpochCommitmentReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentReceivedEvent) {
		p.chainManager.ProcessCommitmentFromSource(event.Commitment, event.Source)
	}))

	p.networkProtocol.Events.EpochCommitmentRequestReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentRequestReceivedEvent) {
		if requestedCommitment, _ := p.chainManager.Commitment(event.CommitmentID); requestedCommitment != nil && requestedCommitment.Commitment() != nil {
			p.networkProtocol.SendEpochCommitment(requestedCommitment.Commitment(), event.Source)
		}
	}))

	p.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		p.networkProtocol.SendBlock(block.ModelsBlock)
	}))

	p.Events.Engine.BlockRequester.Tick.Attach(event.NewClosure(func(blockID models.BlockID) {
		p.networkProtocol.RequestBlock(blockID)
	}))

	p.chainManager.CommitmentRequester.Events.Tick.Attach(event.NewClosure(func(commitmentID commitment.ID) {
		p.networkProtocol.RequestCommitment(commitmentID)
	}))

	p.networkProtocol.Events.AttestationsRequestReceived.Attach(event.NewClosure(func(event *network.AttestationsRequestReceivedEvent) {
		p.ProcessAttestationsRequest(event.StartIndex, event.EndIndex, event.Source)
	}))

	p.networkProtocol.Events.AttestationsReceived.Attach(event.NewClosure(func(event *network.AttestationsReceivedEvent) {
		p.ProcessAttestations(event.Attestations, event.Source)
	}))
}

func (p *Protocol) initChainManager() {
	p.chainManager = chainmanager.NewManager(p.Engine().Storage.Settings.LatestCommitment())

	p.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		p.chainManager.ProcessCommitment(details.Commitment)
	}))

	p.Events.Engine.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.chainManager.CommitmentRequester.EvictUntil(epochIndex)
	}))

	p.Events.Engine.EvictionState.EpochEvicted.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.chainManager.Evict(epochIndex)
	}))

	p.chainManager.Events.ForkDetected.Attach(event.NewClosure(func(event *chainmanager.ForkDetectedEvent) {
		p.onForkDetected(event.Commitment, event.StartEpoch(), event.EndEpoch(), event.Source)
	}))
}

func (p *Protocol) onForkDetected(commitment *commitment.Commitment, startIndex epoch.Index, endIndex epoch.Index, source identity.ID) bool {
	claimedWeight := commitment.CumulativeWeight()
	mainChainCommitment, err := p.Engine().Storage.Commitments.Load(commitment.Index())
	if err != nil {
		p.Events.Error.Trigger(errors.Errorf("failed to load commitment for main chain tip at index %d", commitment.Index()))
		return true
	}

	mainChainWeight := mainChainCommitment.CumulativeWeight()

	if claimedWeight <= mainChainWeight {
		// TODO: ban source?
		p.Events.Error.Trigger(errors.Errorf("dot not process fork with %d CW <= than main chain %d CW received from %s", claimedWeight, mainChainWeight, source))
		return true
	}

	p.networkProtocol.RequestAttestations(startIndex, endIndex, source)
	return false
}

func (p *Protocol) switchEngines() {
	p.activeEngineMutex.Lock()

	// Save a reference to the current main engine and storage so that we can shut it down and prune it after switching
	oldEngine := p.mainEngine

	if err := p.engineManager.SetActiveInstance(p.candidateEngine); err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error switching engines"))
		p.activeEngineMutex.Unlock()
		return
	}

	p.mainEngine = p.candidateEngine
	p.candidateEngine = nil

	p.linkTo(p.mainEngine)
	//TODO: check if we need to switch the chainManager or reset it somehow

	p.activeEngineMutex.Unlock()

	// Shutdown old engine and storage
	oldEngine.Shutdown()

	// Cleanup filesystem
	if err := oldEngine.RemoveFromFilesystem(); err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error removing storage directory after switching engines"))
	}
}

func (p *Protocol) initTipManager() {
	// TODO: SWITCH ENGINE SIMILAR TO REQUESTER
	p.TipManager = tipmanager.New(p.CongestionControl.Block, p.optsTipManagerOptions...)

	p.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		p.TipManager.AddTip(block)
	}))

	p.Events.Engine.EvictionState.EpochEvicted.Attach(event.NewClosure(func(index epoch.Index) {
		p.TipManager.EvictTSCCache(index)
	}))

	p.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		p.TipManager.RemoveStrongParents(block.ModelsBlock)
	}))

	p.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		if schedulerBlock, exists := p.CongestionControl.Block(block.ID()); exists {
			p.TipManager.DeleteTip(schedulerBlock)
		}
	}))

	p.Events.Engine.Tangle.BlockDAG.BlockUnorphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		if schedulerBlock, exists := p.CongestionControl.Block(block.ID()); exists {
			p.TipManager.AddTip(schedulerBlock)
		}
	}))

	p.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		p.TipManager.PromoteFutureTips(details.Commitment)
	}))

	p.Events.Engine.EvictionState.EpochEvicted.Attach(event.NewClosure(func(index epoch.Index) {
		p.TipManager.Evict(index)
	}))

	p.Events.TipManager = p.TipManager.Events
}

func (p *Protocol) ProcessBlock(block *models.Block, src identity.ID) error {
	mainEngine := p.MainEngineInstance()

	isSolid, chain, _ := p.chainManager.ProcessCommitmentFromSource(block.Commitment(), src)
	if !isSolid {
		return errors.Errorf("Chain is not solid: %s\nLatest commitment: %s\nBlock ID: %s", block.Commitment().ID(), mainEngine.Storage.Settings.LatestCommitment().ID(), block.ID())
	}

	if mainChain := mainEngine.Storage.Settings.ChainID(); chain.ForkingPoint.ID() == mainChain {
		mainEngine.Engine.ProcessBlockFromPeer(block, src)
		return nil
	}

	if candidateEngine := p.CandidateEngineInstance(); candidateEngine != nil {
		if candidateChain := candidateEngine.Storage.Settings.ChainID(); chain.ForkingPoint.ID() == candidateChain {
			candidateEngine.Engine.ProcessBlockFromPeer(block, src)

			if candidateEngine.Engine.IsBootstrapped() && candidateEngine.Storage.Settings.LatestCommitment().CumulativeWeight() > mainEngine.Storage.Settings.LatestCommitment().CumulativeWeight() {
				p.switchEngines()
			}
			return nil
		}
	}
	return errors.Errorf("block was not processed.")
}

func (p *Protocol) ProcessAttestationsRequest(startIndex epoch.Index, endIndex epoch.Index, src identity.ID) {
	mainEngine := p.MainEngineInstance()

	if mainEngine.Engine.NotarizationManager.Attestations.LastCommittedEpoch() < endIndex {
		// Invalid request received from src
		// TODO: ban peer?
		return
	}

	attestations := orderedmap.New[epoch.Index, *set.AdvancedSet[*notarization.Attestation]]()
	for i := startIndex; i <= endIndex; i++ {
		attestationsForEpoch, err := mainEngine.Engine.NotarizationManager.Attestations.Get(i)
		if err != nil {
			p.Events.Error.Trigger(errors.Wrapf(err, "failed to get attestations for epoch %d upon request", i))
			return
		}

		attestationsSet := set.NewAdvancedSet[*notarization.Attestation]()
		if err := attestationsForEpoch.Stream(func(_ identity.ID, attestation *notarization.Attestation) bool {
			attestationsSet.Add(attestation)
			return true
		}); err != nil {
			p.Events.Error.Trigger(errors.Wrapf(err, "failed to stream attestations for epoch %d", i))
			return
		}

		attestations.Set(i, attestationsSet)
	}

	p.networkProtocol.SendAttestations(attestations, src)
}

func (p *Protocol) ProcessAttestations(attestations *orderedmap.OrderedMap[epoch.Index, *set.AdvancedSet[*notarization.Attestation]], source identity.ID) {
	if attestations.Size() == 0 {
		p.Events.Error.Trigger(errors.Errorf("received attestations from peer %s are empty", source.String()))
		return
	}

	var attestationEpochs []epoch.Index
	attestations.ForEach(func(key epoch.Index, value *set.AdvancedSet[*notarization.Attestation]) bool {
		attestationEpochs = append(attestationEpochs, key)
		return true
	})

	sort.Slice(attestationEpochs, func(i, j int) bool {
		return attestationEpochs[i] < attestationEpochs[j]
	})

	firstEpochAttestations, exists := attestations.Get(attestationEpochs[0])
	if !exists {
		p.Events.Error.Trigger(errors.Errorf("received attestations from peer %s are empty", source.String()))
	}

	attestationsForEpoch, _, exists := firstEpochAttestations.Head()
	if !exists {
		p.Events.Error.Trigger(errors.Errorf("received attestations from peer %s are empty", source.String()))
	}

	forkedEvent, exists := p.chainManager.ForkedEventByForkingPoint(attestationsForEpoch.CommitmentID)
	if !exists {
		p.Events.Error.Trigger(errors.Errorf("failed to get forking point for commitment %s", attestationsForEpoch.CommitmentID))
		return
	}

	mainEngine := p.MainEngineInstance()

	// Obtain mana vector at forking point - 1
	snapshotTargetIndex := forkedEvent.Chain.ForkingPoint.ID().Index() - 1
	wb := sybilprotection.NewWeightsBatch(snapshotTargetIndex)

	var calculatedCumulativeWeight int64
	mainEngine.Engine.NotarizationManager.PerformLocked(func(m *notarization.Manager) {
		// Calculate the difference between the latest commitment ledger and the ledger at the snapshot target index
		latestCommitment := mainEngine.Storage.Settings.LatestCommitment()
		for i := latestCommitment.Index(); i >= snapshotTargetIndex; i-- {
			if err := mainEngine.Engine.LedgerState.StateDiffs.StreamSpentOutputs(i, func(output *ledger.OutputWithMetadata) error {
				if iotaBalance, balanceExists := output.IOTABalance(); balanceExists {
					wb.Update(output.ConsensusManaPledgeID(), int64(iotaBalance))
				}
				return nil
			}); err != nil {
				p.Events.Error.Trigger(errors.Wrap(err, "error streaming spent outputs for processing attestations"))
				return
			}

			if err := mainEngine.Engine.LedgerState.StateDiffs.StreamCreatedOutputs(i, func(output *ledger.OutputWithMetadata) error {
				if iotaBalance, balanceExists := output.IOTABalance(); balanceExists {
					wb.Update(output.ConsensusManaPledgeID(), -int64(iotaBalance))
				}
				return nil
			}); err != nil {
				p.Events.Error.Trigger(errors.Wrap(err, "error streaming created outputs for processing attestations"))
				return
			}
		}

		// Get our cumulative weight at the snapshot target index and apply all the received attestations on stop while verifying the validity of each signature
		calculatedCumulativeWeight = lo.PanicOnErr(mainEngine.Storage.Commitments.Load(snapshotTargetIndex)).CumulativeWeight()
		visitedIdentities := make(map[identity.ID]types.Empty)
		for epochIndex := forkedEvent.StartEpoch(); epochIndex <= forkedEvent.EndEpoch(); epochIndex++ {
			epochAttestations, epochExists := attestations.Get(epochIndex)
			if !epochExists {
				p.Events.Error.Trigger(errors.Errorf("attestations for epoch %d missing", epochIndex))
				//TODO: ban source?
				return
			}
			for it := epochAttestations.Iterator(); it.HasNext(); {
				attestation := it.Next()

				if valid, err := attestation.VerifySignature(); !valid {
					if err != nil {
						p.Events.Error.Trigger(errors.Wrapf(err, "error validating attestation signature provided by %s", source))
						return
					}

					p.Events.Error.Trigger(errors.Errorf("invalid attestation signature provided by %s", source))
					return
				}

				issuerID := attestation.IssuerID()
				if _, alreadyVisited := visitedIdentities[issuerID]; alreadyVisited {
					p.Events.Error.Trigger(errors.Errorf("invalid attestation from source %s, issuerID %s contains multiple attestations", source, issuerID))
					//TODO: ban source!
					return
				}

				if weight, weightExists := mainEngine.Engine.SybilProtection.Weights().Get(issuerID); weightExists {
					calculatedCumulativeWeight += weight.Value
				}
				calculatedCumulativeWeight += wb.Get(issuerID)

				visitedIdentities[issuerID] = types.Void
			}
		}
	})

	// Compare the calculated cumulative weight with the current one of our current change to verify it is really higher
	if calculatedCumulativeWeight <= mainEngine.Storage.Settings.LatestCommitment().CumulativeWeight() {
		p.Events.Error.Trigger(errors.Errorf("forking point does not accumulate enough weight %d CW <= main chain %d CW", calculatedCumulativeWeight, p.Engine().Storage.Settings.LatestCommitment().CumulativeWeight()))
		return
	}

	candidateEngine, err := p.engineManager.ForkEngineAtEpoch(snapshotTargetIndex)
	if err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error creating new candidate engine"))
		return
	}

	// Set the engine as the new candidate
	p.activeEngineMutex.Lock()
	p.candidateEngine = candidateEngine
	p.activeEngineMutex.Unlock()
}

func (p *Protocol) Engine() *engine.Engine {
	// This getter is for backwards compatibility, can be refactored out later on
	return p.MainEngineInstance().Engine
}

func (p *Protocol) MainEngineInstance() *enginemanager.EngineInstance {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.mainEngine
}

func (p *Protocol) CandidateEngineInstance() (instance *enginemanager.EngineInstance) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.candidateEngine
}

func (p *Protocol) ChainManager() (instance *chainmanager.Manager) {
	return p.chainManager
}

func (p *Protocol) linkTo(engineInstance *enginemanager.EngineInstance) {
	p.Events.Engine.LinkTo(engineInstance.Engine.Events)
	p.TipManager.LinkTo(engineInstance.Engine)
	p.CongestionControl.LinkTo(engineInstance.Engine)
}

func (p *Protocol) Network() *network.Protocol {
	return p.networkProtocol
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBaseDirectory(baseDirectory string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsBaseDirectory = baseDirectory
	}
}

func WithPruningThreshold(pruningThreshold uint64) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsPruningThreshold = pruningThreshold
	}
}

func WithSnapshotPath(snapshot string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsSnapshotPath = snapshot
	}
}

func WithSybilProtectionProvider(sybilProtectionProvider engine.ModuleProvider[sybilprotection.SybilProtection]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsSybilProtectionProvider = sybilProtectionProvider
	}
}

func WithThroughputQuotaProvider(sybilProtectionProvider engine.ModuleProvider[throughputquota.ThroughputQuota]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsThroughputQuotaProvider = sybilProtectionProvider
	}
}

func WithCongestionControlOptions(opts ...options.Option[congestioncontrol.CongestionControl]) options.Option[Protocol] {
	return func(e *Protocol) {
		e.optsCongestionControlOptions = opts
	}
}

func WithTipManagerOptions(opts ...options.Option[tipmanager.TipManager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsTipManagerOptions = opts
	}
}

// func WithSolidificationOptions(opts ...options.Option[solidification.Requester]) options.Option[Protocol] {
// 	return func(n *Protocol) {
// 		n.optsSolidificationOptions = opts
// 	}
// }

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsEngineOptions = opts
	}
}

func WithStorageDatabaseManagerOptions(opts ...options.Option[database.Manager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsStorageDatabaseManagerOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
