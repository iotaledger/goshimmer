package protocol

import (
	"fmt"
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
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
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

	p.Events.Network = p.networkProtocol.Events

	p.Events.Network.BlockRequestReceived.Attach(event.NewClosure(func(event *network.BlockRequestReceivedEvent) {
		if block, exists := p.MainEngineInstance().Engine.Block(event.BlockID); exists {
			p.networkProtocol.SendBlock(block, event.Source)
		}
	}))

	p.Events.Network.BlockReceived.Attach(event.NewClosure(func(event *network.BlockReceivedEvent) {
		if err := p.ProcessBlock(event.Block, event.Source); err != nil {
			p.Events.Error.Trigger(err)
		}
	}))

	p.Events.Network.EpochCommitmentReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentReceivedEvent) {
		p.chainManager.ProcessCommitmentFromSource(event.Commitment, event.Source)
	}))

	p.Events.Network.EpochCommitmentRequestReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentRequestReceivedEvent) {
		// when we receive a commitment request, do not look it up in the ChainManager but in the storage, else we might answer with commitments we did not issue ourselves and for which we cannot provide attestations
		if requestedCommitment, err := p.Engine().Storage.Commitments.Load(event.CommitmentID.Index()); err == nil && requestedCommitment.ID() == event.CommitmentID {
			p.networkProtocol.SendEpochCommitment(requestedCommitment, event.Source)
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

	p.Events.Network.AttestationsRequestReceived.Attach(event.NewClosure(func(event *network.AttestationsRequestReceivedEvent) {
		p.ProcessAttestationsRequest(event.Commitment, event.EndIndex, event.Source)
	}))

	p.Events.Network.AttestationsReceived.Attach(event.NewClosure(func(event *network.AttestationsReceivedEvent) {
		p.ProcessAttestations(event.Commitment, event.BlockIDs, event.Attestations, event.Source)
	}))
}

func (p *Protocol) initChainManager() {
	p.chainManager = chainmanager.NewManager(p.Engine().Storage.Settings.LatestCommitment())
	p.Events.ChainManager = p.chainManager.Events

	p.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		p.chainManager.ProcessCommitment(details.Commitment)
	}))

	p.Events.Engine.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.chainManager.CommitmentRequester.EvictUntil(epochIndex)
	}))

	p.Events.Engine.EvictionState.EpochEvicted.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.chainManager.Evict(epochIndex)
	}))

	p.Events.ChainManager.ForkDetected.Attach(event.NewClosure(func(event *chainmanager.ForkDetectedEvent) {
		p.onForkDetected(event.Commitment, event.ForkingPoint(), event.EndEpoch(), event.Source)
	}))
}

func (p *Protocol) onForkDetected(commitment *commitment.Commitment, forkingPoint *commitment.Commitment, endIndex epoch.Index, source identity.ID) bool {
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

	p.networkProtocol.RequestAttestations(forkingPoint, endIndex, source)
	return false
}

func (p *Protocol) switchEngines() {
	p.activeEngineMutex.Lock()

	if p.candidateEngine == nil {
		p.activeEngineMutex.Unlock()
		return
	}

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

	p.Events.MainEngineSwitched.Trigger(p.MainEngineInstance())

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
		return errors.Errorf("protocol ProcessBlock failed. chain is not solid: %s, latest commitment: %s, block ID: %s", block.Commitment().ID(), mainEngine.Storage.Settings.LatestCommitment().ID(), block.ID())
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
	return errors.Errorf("block from source %s was not processed: %s", src, block.ID())
}

func (p *Protocol) ProcessAttestationsRequest(forkingPoint *commitment.Commitment, endIndex epoch.Index, src identity.ID) {
	mainEngine := p.MainEngineInstance()

	if mainEngine.Engine.NotarizationManager.Attestations.LastCommittedEpoch() < endIndex {
		// Invalid request received from src
		// TODO: ban peer?
		return
	}

	blockIDs := models.NewBlockIDs()
	attestations := orderedmap.New[epoch.Index, *set.AdvancedSet[*notarization.Attestation]]()
	for i := forkingPoint.Index(); i <= endIndex; i++ {
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

		if err := mainEngine.Engine.Storage.Blocks.ForEachBlockInEpoch(i, func(blockID models.BlockID) bool {
			blockIDs.Add(blockID)
			return true
		}); err != nil {
			p.Events.Error.Trigger(errors.Wrap(err, "failed to read blocks from epoch"))
			return
		}
	}

	p.networkProtocol.SendAttestations(forkingPoint, blockIDs, attestations, src)
}

func (p *Protocol) ProcessAttestations(forkingPoint *commitment.Commitment, blockIDs models.BlockIDs, attestations *orderedmap.OrderedMap[epoch.Index, *set.AdvancedSet[*notarization.Attestation]], source identity.ID) {
	fmt.Println("Received attestations for", forkingPoint.ID())
	if attestations.Size() == 0 {
		p.Events.Error.Trigger(errors.Errorf("received attestations from peer %s are empty", source.String()))
		return
	}

	forkedEvent, exists := p.chainManager.ForkedEventByForkingPoint(forkingPoint.ID())
	if !exists {
		p.Events.Error.Trigger(errors.Errorf("failed to get forking point for commitment %s", forkingPoint.ID()))
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
		for epochIndex := forkedEvent.ForkingPoint().Index(); epochIndex <= forkedEvent.EndEpoch(); epochIndex++ {
			epochAttestations, epochExists := attestations.Get(epochIndex)
			if !epochExists {
				p.Events.Error.Trigger(errors.Errorf("attestations for epoch %d missing", epochIndex))
				//TODO: ban source?
				return
			}
			visitedIdentities := make(map[identity.ID]types.Empty)
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

	// Compare the calculated cumulative weight with ours to verify it is really higher
	weightAtForkedEventEnd := lo.PanicOnErr(mainEngine.Storage.Commitments.Load(forkedEvent.EndEpoch())).CumulativeWeight()
	if calculatedCumulativeWeight <= weightAtForkedEventEnd {
		forkedEventClaimedWeight := forkedEvent.Commitment.CumulativeWeight()
		forkedEventMainWeight := lo.PanicOnErr(mainEngine.Engine.Storage.Commitments.Load(forkedEvent.Commitment.Index())).CumulativeWeight()
		p.Events.Error.Trigger(errors.Errorf("fork at point %d does not accumulate enough weight at epoch %d calculated %d CW <= main chain %d CW. fork event detected at %d was %d CW > %d CW",
			forkedEvent.ForkingPoint().Index(),
			forkedEvent.EndEpoch(),
			calculatedCumulativeWeight,
			weightAtForkedEventEnd,
			forkedEvent.Commitment.Index(),
			forkedEventClaimedWeight,
			forkedEventMainWeight))
		//TODO: ban source?
		return
	}

	candidateEngine, err := p.engineManager.ForkEngineAtEpoch(snapshotTargetIndex)
	if err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error creating new candidate engine"))
		return
	}

	// Set the chain to the correct forking point
	if err := candidateEngine.Engine.Storage.Settings.SetChainID(forkedEvent.Chain.ForkingPoint.ID()); err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error setting the ChainID on the forked engine"))
		candidateEngine.Shutdown()
		candidateEngine.RemoveFromFilesystem()
		return
	}

	// Attach the engine block requests to the protocol and detach as soon as we switch to that engine
	requestBlocks := event.NewClosure(func(blockID models.BlockID) {
		p.networkProtocol.RequestBlock(blockID)
	})
	candidateEngine.Engine.Events.BlockRequester.Tick.Attach(requestBlocks)
	p.Events.MainEngineSwitched.Hook(event.NewClosure(func(_ *enginemanager.EngineInstance) {
		candidateEngine.Engine.Events.BlockRequester.Tick.Detach(requestBlocks)
	}), 1)

	// Add all the blocks from the forking point to the requester since those will not be passed to the engine by the protocol
	requestedBlockIDs := memstorage.New[models.BlockID, types.Empty]()
	for _, b := range blockIDs.Slice() {
		fmt.Println("> Add", b)
		requestedBlockIDs.Set(b, types.Void)
	}
	if requestedBlockIDs.Size() > 0 {
		// Only attach a closure if there really are blocks that need requesting
		var processRequestedBlock *event.Closure[*network.BlockReceivedEvent]
		processRequestedBlock = event.NewClosure(func(event *network.BlockReceivedEvent) {
			if requestedBlockIDs.Delete(event.Block.ID()) {
				fmt.Println("Process requested block in candidate engine:", event.Block.ID())
				candidateEngine.Engine.ProcessBlockFromPeer(event.Block, event.Source)

				if requestedBlockIDs.IsEmpty() {
					p.Events.Network.BlockReceived.Detach(processRequestedBlock)
					fmt.Println("Last requested block received")
				}
			}
		})
		// Attach the block received since we want to pass the received blocks to out engine directly
		p.Events.Network.BlockReceived.Attach(processRequestedBlock)
	}

	requestedBlockIDs.ForEachKey(func(id models.BlockID) bool {
		fmt.Println("> Request", id)
		candidateEngine.Engine.BlockRequester.StartTicker(id)
		return true
	})
	fmt.Printf(">>> Added %d blocks to the candidate engine BlockRequester\n", requestedBlockIDs.Size())

	// Set the engine as the new candidate
	p.activeEngineMutex.Lock()
	p.candidateEngine = candidateEngine
	p.activeEngineMutex.Unlock()

	p.Events.CandidateEngineActivated.Trigger(candidateEngine)
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
