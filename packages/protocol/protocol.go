package protocol

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock/blocktime"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/tangleconsensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter/blockfilter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxoledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization/slotnotarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/inmemorytangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/orderedmap"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events            *Events
	CongestionControl *congestioncontrol.CongestionControl
	TipManager        *tipmanager.TipManager
	chainManager      *chainmanager.Manager
	engineManager     *enginemanager.EngineManager

	Workers         *workerpool.Group
	dispatcher      network.Endpoint
	networkProtocol *network.Protocol

	activeEngineMutex sync.RWMutex
	mainEngine        *engine.Engine
	candidateEngine   *engine.Engine

	optsBaseDirectory    string
	optsSnapshotPath     string
	optsPruningThreshold uint64

	optsCongestionControlOptions      []options.Option[congestioncontrol.CongestionControl]
	optsEngineOptions                 []options.Option[engine.Engine]
	optsChainManagerOptions           []options.Option[chainmanager.Manager]
	optsTipManagerOptions             []options.Option[tipmanager.TipManager]
	optsStorageDatabaseManagerOptions []options.Option[database.Manager]

	optsClockProvider           module.Provider[*engine.Engine, clock.Clock]
	optsLedgerProvider          module.Provider[*engine.Engine, ledger.Ledger]
	optsFilterProvider          module.Provider[*engine.Engine, filter.Filter]
	optsSybilProtectionProvider module.Provider[*engine.Engine, sybilprotection.SybilProtection]
	optsThroughputQuotaProvider module.Provider[*engine.Engine, throughputquota.ThroughputQuota]
	optsNotarizationProvider    module.Provider[*engine.Engine, notarization.Notarization]
	optsTangleProvider          module.Provider[*engine.Engine, tangle.Tangle]
	optsConsensusProvider       module.Provider[*engine.Engine, consensus.Consensus]
}

func New(workers *workerpool.Group, dispatcher network.Endpoint, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events:                      NewEvents(),
		Workers:                     workers,
		dispatcher:                  dispatcher,
		optsClockProvider:           blocktime.NewProvider(),
		optsLedgerProvider:          utxoledger.NewProvider(),
		optsFilterProvider:          blockfilter.NewProvider(),
		optsSybilProtectionProvider: dpos.NewProvider(),
		optsThroughputQuotaProvider: mana1.NewProvider(),
		optsNotarizationProvider:    slotnotarization.NewProvider(),
		optsTangleProvider:          inmemorytangle.NewProvider(),
		optsConsensusProvider:       tangleconsensus.NewProvider(),

		optsBaseDirectory:    "",
		optsPruningThreshold: 6 * 60, // 1 hour given that slot duration is 10 seconds
	}, opts,
		(*Protocol).initNetworkEvents,
		(*Protocol).initEngineManager,
		(*Protocol).initCongestionControl,
		(*Protocol).initChainManager,
		(*Protocol).initTipManager,
	)
}

// Run runs the protocol.
func (p *Protocol) Run() {
	p.Events.Engine.LinkTo(p.mainEngine.Events)

	if err := p.mainEngine.Initialize(p.optsSnapshotPath); err != nil {
		panic(err)
	}

	p.linkTo(p.mainEngine)
	p.networkProtocol = network.NewProtocol(p.dispatcher, p.Workers.CreatePool("NetworkProtocol"), p.SlotTimeProvider()) // Use max amount of workers for networking
	p.Events.Network.LinkTo(p.networkProtocol.Events)
}

func (p *Protocol) Shutdown() {
	if p.networkProtocol != nil {
		p.networkProtocol.Unregister()
	}

	p.CongestionControl.Shutdown()

	p.chainManager.CommitmentRequester.Shutdown()

	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	p.mainEngine.Shutdown()
	if p.candidateEngine != nil {
		p.candidateEngine.Shutdown()
	}

	p.Workers.Shutdown()
}

func (p *Protocol) initEngineManager() {
	p.engineManager = enginemanager.New(
		p.Workers.CreateGroup("EngineManager"),
		p.optsBaseDirectory,
		DatabaseVersion,
		p.optsStorageDatabaseManagerOptions,
		p.optsEngineOptions,
		p.optsClockProvider,
		p.optsLedgerProvider,
		p.optsFilterProvider,
		p.optsSybilProtectionProvider,
		p.optsThroughputQuotaProvider,
		p.optsNotarizationProvider,
		p.optsTangleProvider,
		p.optsConsensusProvider,
	)

	p.Events.Engine.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
		p.Engine().Storage.PruneUntilSlot(index - slot.Index(p.optsPruningThreshold))
	}, event.WithWorkerPool(p.Workers.CreatePool("PruneEngine", 2)))

	p.mainEngine = lo.PanicOnErr(p.engineManager.LoadActiveEngine())
}

func (p *Protocol) initCongestionControl() {
	p.CongestionControl = congestioncontrol.New(p.optsCongestionControlOptions...)
	p.Events.CongestionControl.LinkTo(p.CongestionControl.Events)
}

func (p *Protocol) initNetworkEvents() {
	wpBlocks := p.Workers.CreatePool("NetworkEvents.Blocks") // Use max amount of workers for sending, receiving and requesting blocks
	p.Events.Network.BlockRequestReceived.Hook(func(event *network.BlockRequestReceivedEvent) {
		if block, exists := p.MainEngineInstance().Block(event.BlockID); exists {
			p.networkProtocol.SendBlock(block, event.Source)
		}
	}, event.WithWorkerPool(wpBlocks))
	p.Events.Engine.BlockRequester.Tick.Hook(func(blockID models.BlockID) {
		p.networkProtocol.RequestBlock(blockID)
	}, event.WithWorkerPool(wpBlocks))
	p.Events.CongestionControl.Scheduler.BlockScheduled.Hook(func(block *scheduler.Block) {
		p.networkProtocol.SendBlock(block.ModelsBlock)
	}, event.WithWorkerPool(wpBlocks))
	p.Events.Network.BlockReceived.Hook(func(event *network.BlockReceivedEvent) {
		if err := p.ProcessBlock(event.Block, event.Source); err != nil {
			p.Events.Error.Trigger(err)
		}
	}, event.WithWorkerPool(wpBlocks))

	wpCommitments := p.Workers.CreatePool("NetworkEvents.SlotCommitments", 1) // Using just 1 worker to avoid contention
	p.Events.Network.SlotCommitmentRequestReceived.Hook(func(event *network.SlotCommitmentRequestReceivedEvent) {
		// when we receive a commitment request, do not look it up in the ChainManager but in the storage, else we might answer with commitments we did not issue ourselves and for which we cannot provide attestations
		if requestedCommitment, err := p.Engine().Storage.Commitments.Load(event.CommitmentID.Index()); err == nil && requestedCommitment.ID() == event.CommitmentID {
			p.networkProtocol.SendSlotCommitment(requestedCommitment, event.Source)
		}
	}, event.WithWorkerPool(wpCommitments))

	wpAttestations := p.Workers.CreatePool("NetworkEvents.Attestations", 1) // Using just 1 worker to avoid contention
	p.Events.Network.AttestationsRequestReceived.Hook(func(event *network.AttestationsRequestReceivedEvent) {
		p.ProcessAttestationsRequest(event.Commitment, event.EndIndex, event.Source)
	}, event.WithWorkerPool(wpAttestations))
	p.Events.Network.AttestationsReceived.Hook(func(event *network.AttestationsReceivedEvent) {
		p.ProcessAttestations(event.Commitment, event.BlockIDs, event.Attestations, event.Source)
	}, event.WithWorkerPool(wpAttestations))
}

func (p *Protocol) initChainManager() {
	p.chainManager = chainmanager.NewManager(p.optsChainManagerOptions...)

	// the earliestRootCommitment is used to make sure that the chainManager knows the earliest possible
	// commitment that blocks we are solidifying will refer to. Failing to do so will prevent those blocks
	// from being processed as their chain will be deemed unsolid.
	earliestRootCommitment := func() *commitment.Commitment {
		earliestRootCommitmentID := p.Engine().EvictionState.EarliestRootCommitmentID()
		rootCommitment, err := p.Engine().Storage.Commitments.Load(earliestRootCommitmentID.Index())
		if err != nil {
			panic(fmt.Sprintln("could not load earliest commitment after engine initialization", err))
		}
		return rootCommitment
	}

	p.Engine().HookInitialized(func() {
		rootCommitment := earliestRootCommitment()

		// the rootCommitment is also the earliest point in the chain we can fork from. It is used to prevent
		// solidifying and processing commitments that we won't be able to switch to.
		if err := p.Engine().Storage.Settings.SetChainID(rootCommitment.ID()); err != nil {
			panic(fmt.Sprintln("could not load set main engine's chain using", rootCommitment))
		}
		p.chainManager.Initialize(rootCommitment)
	})

	p.Events.ChainManager = p.chainManager.Events

	wp := p.Workers.CreatePool("ChainManager", 1) // Using just 1 worker to avoid contention

	p.Events.Engine.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
		p.chainManager.ProcessCommitment(details.Commitment)
	}, event.WithWorkerPool(wp))

	p.Events.Engine.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
		rootCommitment := earliestRootCommitment()

		// It is essential that we set the rootCommitment before evicting the chainManager's state, this way
		// we first specify the chain's cut-off point, and only then evict the state. It is also important to
		// note that no multiple goroutines should be allowed to perform this operation at once, hence the
		// hooking worker pool should always have a single worker or these two calls should be protected by a lock.
		p.chainManager.SetRootCommitment(rootCommitment)

		// We want to evict just below the height of our new root commitment (so that the slot of the root commitment
		// stays in memory storage and with it the root commitment itself as well).
		p.chainManager.EvictUntil(rootCommitment.ID().Index() - 1)

		// We don't want to request any commitments that are equal or below the new root commitment index anymore.
		p.chainManager.CommitmentRequester.EvictUntil(rootCommitment.ID().Index())
	}, event.WithWorkerPool(wp))
	p.Events.ChainManager.ForkDetected.Hook(p.onForkDetected, event.WithWorkerPool(wp))
	p.Events.Network.SlotCommitmentReceived.Hook(func(event *network.SlotCommitmentReceivedEvent) {
		p.chainManager.ProcessCommitmentFromSource(event.Commitment, event.Source)
	}, event.WithWorkerPool(wp))
	p.chainManager.CommitmentRequester.Events.Tick.Hook(func(commitmentID commitment.ID) {
		// Check if we have the requested commitment in our storage before asking our peers for it.
		// This can happen after we restart the node because the chain manager builds up the chain again.
		if cm, _ := p.Engine().Storage.Commitments.Load(commitmentID.Index()); cm != nil {
			if cm.ID() == commitmentID {
				wp.Submit(func() {
					p.chainManager.ProcessCommitment(cm)
				})
				return
			}
		}

		p.networkProtocol.RequestCommitment(commitmentID)
	}, event.WithWorkerPool(p.Workers.CreatePool("RequestCommitment", 2)))
}

func (p *Protocol) initTipManager() {
	p.TipManager = tipmanager.New(p.Workers.CreateGroup("TipManager"), p.CongestionControl.Block, p.optsTipManagerOptions...)
	p.Events.TipManager = p.TipManager.Events

	wp := p.Workers.CreatePool("TipManagerAttach", 1) // Using just 1 worker to avoid contention

	p.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
		if schedulerBlock, exists := p.CongestionControl.Block(block.ID()); exists {
			p.TipManager.DeleteTip(schedulerBlock)
		}
	})
	p.Events.Engine.Tangle.BlockDAG.BlockUnorphaned.Hook(func(block *blockdag.Block) {
		if schedulerBlock, exists := p.CongestionControl.Block(block.ID()); exists {
			p.TipManager.AddTip(schedulerBlock)
		}
	})
	p.Events.CongestionControl.Scheduler.BlockScheduled.Hook(func(block *scheduler.Block) {
		p.TipManager.AddTip(block)
	}, event.WithWorkerPool(wp))
	p.Events.Engine.EvictionState.SlotEvicted.Hook(func(index slot.Index) {
		p.TipManager.EvictTSCCache(index)
	}, event.WithWorkerPool(wp))
	p.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		p.TipManager.RemoveStrongParents(block.ModelsBlock)
	}, event.WithWorkerPool(wp))
}

func (p *Protocol) onForkDetected(fork *chainmanager.Fork) {
	claimedWeight := fork.Commitment.CumulativeWeight()
	mainChainCommitment, err := p.Engine().Storage.Commitments.Load(fork.Commitment.Index())
	if err != nil {
		p.Events.Error.Trigger(errors.Errorf("failed to load commitment for main chain tip at index %d", fork.Commitment.Index()))
		return
	}

	mainChainWeight := mainChainCommitment.CumulativeWeight()

	if claimedWeight <= mainChainWeight {
		// TODO: ban source?
		p.Events.Error.Trigger(errors.Errorf("dot not process fork with %d CW <= than main chain %d CW received from %s", claimedWeight, mainChainWeight, fork.Source))
		return
	}

	p.networkProtocol.RequestAttestations(fork.ForkingPoint, fork.Commitment.Index(), fork.Source)
}

func (p *Protocol) switchEngines() {
	p.activeEngineMutex.Lock()

	if p.candidateEngine == nil {
		p.activeEngineMutex.Unlock()
		return
	}

	// Try to re-org the chain manager
	if err := p.chainManager.SwitchMainChain(p.candidateEngine.Storage.Settings.LatestCommitment().ID()); err != nil {
		p.activeEngineMutex.Unlock()
		p.Events.Error.Trigger(errors.Wrap(err, "switching main chain failed"))
		return
	}

	if err := p.engineManager.SetActiveInstance(p.candidateEngine); err != nil {
		p.activeEngineMutex.Unlock()
		p.Events.Error.Trigger(errors.Wrap(err, "error switching engines"))
		return
	}

	// Stop current Scheduler
	p.CongestionControl.Shutdown()

	p.Events.Engine.LinkTo(p.candidateEngine.Events)
	p.linkTo(p.candidateEngine)

	// Save a reference to the current main engine and storage so that we can shut it down and prune it after switching
	oldEngine := p.mainEngine
	oldEngine.Shutdown()

	p.mainEngine = p.candidateEngine
	p.candidateEngine = nil

	p.activeEngineMutex.Unlock()

	p.Events.MainEngineSwitched.Trigger(p.MainEngineInstance())

	// TODO: copy over old slots from the old engine to the new one

	// Cleanup filesystem
	if err := oldEngine.RemoveFromFilesystem(); err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error removing storage directory after switching engines"))
	}
}

func (p *Protocol) ProcessBlock(block *models.Block, src identity.ID) error {
	mainEngine := p.MainEngineInstance()

	isSolid, chain := p.chainManager.ProcessCommitmentFromSource(block.Commitment(), src)
	if !isSolid {
		if block.Commitment().PrevID() == mainEngine.Storage.Settings.LatestCommitment().ID() {
			return nil
		}

		return errors.Errorf("protocol ProcessBlock failed. chain is not solid: %s, latest commitment: %s, block ID: %s", block.Commitment().ID(), mainEngine.Storage.Settings.LatestCommitment().ID(), block.ID())
	}

	processed := false

	if mainChain := mainEngine.Storage.Settings.ChainID(); chain.ForkingPoint.ID() == mainChain || mainEngine.BlockRequester.HasTicker(block.ID()) {
		mainEngine.ProcessBlockFromPeer(block, src)
		processed = true
	}

	if candidateEngine := p.CandidateEngineInstance(); candidateEngine != nil {
		if candidateChain := candidateEngine.Storage.Settings.ChainID(); chain.ForkingPoint.ID() == candidateChain || candidateEngine.BlockRequester.HasTicker(block.ID()) {
			candidateEngine.ProcessBlockFromPeer(block, src)
			if candidateEngine.IsBootstrapped() &&
				candidateEngine.Storage.Settings.LatestCommitment().Index() >= mainEngine.Storage.Settings.LatestCommitment().Index() &&
				candidateEngine.Storage.Settings.LatestCommitment().CumulativeWeight() > mainEngine.Storage.Settings.LatestCommitment().CumulativeWeight() {
				p.switchEngines()
			}
			processed = true
		}
	}

	if !processed {
		return errors.Errorf("block from source %s was not processed: %s; commits to: %s", src, block.ID(), block.Commitment().ID())
	}

	return nil
}

func (p *Protocol) ProcessAttestationsRequest(forkingPoint *commitment.Commitment, endIndex slot.Index, src identity.ID) {
	mainEngine := p.MainEngineInstance()

	if mainEngine.Notarization.Attestations().LastCommittedSlot() < endIndex {
		// Invalid request received from src
		// TODO: ban peer?
		return
	}

	blockIDs := models.NewBlockIDs()
	attestations := orderedmap.New[slot.Index, *advancedset.AdvancedSet[*notarization.Attestation]]()
	for i := forkingPoint.Index(); i <= endIndex; i++ {
		attestationsForSlot, err := mainEngine.Notarization.Attestations().Get(i)
		if err != nil {
			p.Events.Error.Trigger(errors.Wrapf(err, "failed to get attestations for slot %d upon request", i))
			return
		}

		attestationsSet := advancedset.New[*notarization.Attestation]()
		if err := attestationsForSlot.Stream(func(_ identity.ID, attestation *notarization.Attestation) bool {
			attestationsSet.Add(attestation)
			return true
		}); err != nil {
			p.Events.Error.Trigger(errors.Wrapf(err, "failed to stream attestations for slot %d", i))
			return
		}

		attestations.Set(i, attestationsSet)

		if err := mainEngine.Storage.Blocks.ForEachBlockInSlot(i, func(blockID models.BlockID) bool {
			blockIDs.Add(blockID)
			return true
		}); err != nil {
			p.Events.Error.Trigger(errors.Wrap(err, "failed to read blocks from slot"))
			return
		}
	}

	p.networkProtocol.SendAttestations(forkingPoint, blockIDs, attestations, src)
}

func (p *Protocol) ProcessAttestations(forkingPoint *commitment.Commitment, blockIDs models.BlockIDs, attestations *orderedmap.OrderedMap[slot.Index, *advancedset.AdvancedSet[*notarization.Attestation]], source identity.ID) {
	if attestations.Size() == 0 {
		p.Events.Error.Trigger(errors.Errorf("received attestations from peer %s are empty", source.String()))
		return
	}

	forkedEvent, exists := p.chainManager.ForkByForkingPoint(forkingPoint.ID())
	if !exists {
		p.Events.Error.Trigger(errors.Errorf("failed to get forking point for commitment %s", forkingPoint.ID()))
		return
	}

	mainEngine := p.MainEngineInstance()

	// Obtain mana vector at forking point - 1
	snapshotTargetIndex := forkedEvent.ForkingPoint.Index() - 1
	wb := sybilprotection.NewWeightsBatch(snapshotTargetIndex)

	var calculatedCumulativeWeight int64
	mainEngine.Notarization.PerformLocked(func(n notarization.Notarization) {
		// Calculate the difference between the latest commitment ledger and the ledger at the snapshot target index
		latestCommitment := mainEngine.Storage.Settings.LatestCommitment()
		for i := latestCommitment.Index(); i >= snapshotTargetIndex; i-- {
			if err := mainEngine.Ledger.StateDiffs().StreamSpentOutputs(i, func(output *mempool.OutputWithMetadata) error {
				if iotaBalance, balanceExists := output.IOTABalance(); balanceExists {
					wb.Update(output.ConsensusManaPledgeID(), int64(iotaBalance))
				}
				return nil
			}); err != nil {
				p.Events.Error.Trigger(errors.Wrap(err, "error streaming spent outputs for processing attestations"))
				return
			}

			if err := mainEngine.Ledger.StateDiffs().StreamCreatedOutputs(i, func(output *mempool.OutputWithMetadata) error {
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
		for slotIndex := forkedEvent.ForkingPoint.Index(); slotIndex <= forkedEvent.Commitment.Index(); slotIndex++ {
			slotAttestations, slotExists := attestations.Get(slotIndex)
			if !slotExists {
				p.Events.Error.Trigger(errors.Errorf("attestations for slot %d missing", slotIndex))
				// TODO: ban source?
				return
			}
			visitedIdentities := make(map[identity.ID]types.Empty)
			for it := slotAttestations.Iterator(); it.HasNext(); {
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
					// TODO: ban source!
					return
				}

				if weight, weightExists := mainEngine.SybilProtection.Weights().Get(issuerID); weightExists {
					calculatedCumulativeWeight += weight.Value
				}
				calculatedCumulativeWeight += wb.Get(issuerID)

				visitedIdentities[issuerID] = types.Void
			}
		}
	})

	// Compare the calculated cumulative weight with ours to verify it is really higher
	weightAtForkedEventEnd := lo.PanicOnErr(mainEngine.Storage.Commitments.Load(forkedEvent.Commitment.Index())).CumulativeWeight()
	if calculatedCumulativeWeight <= weightAtForkedEventEnd {
		forkedEventClaimedWeight := forkedEvent.Commitment.CumulativeWeight()
		forkedEventMainWeight := lo.PanicOnErr(mainEngine.Storage.Commitments.Load(forkedEvent.Commitment.Index())).CumulativeWeight()
		p.Events.Error.Trigger(errors.Errorf("fork at point %d does not accumulate enough weight at slot %d calculated %d CW <= main chain %d CW. fork event detected at %d was %d CW > %d CW",
			forkedEvent.ForkingPoint.Index(),
			forkedEvent.Commitment.Index(),
			calculatedCumulativeWeight,
			weightAtForkedEventEnd,
			forkedEvent.Commitment.Index(),
			forkedEventClaimedWeight,
			forkedEventMainWeight))
		// TODO: ban source?
		return
	}

	candidateEngine, err := p.engineManager.ForkEngineAtSlot(snapshotTargetIndex)
	if err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error creating new candidate engine"))
		return
	}

	// Set the chain to the correct forking point
	if err := candidateEngine.Storage.Settings.SetChainID(forkedEvent.ForkingPoint.ID()); err != nil {
		p.Events.Error.Trigger(errors.Wrap(err, "error setting the ChainID on the forked engine"))
		candidateEngine.Shutdown()
		if err := candidateEngine.RemoveFromFilesystem(); err != nil {
			p.Events.Error.Trigger(errors.Wrap(err, "error cleaning up old failed candidate engine from file system"))
		}
		return
	}

	// Attach the engine block requests to the protocol and detach as soon as we switch to that engine
	wp := candidateEngine.Workers.CreatePool("CandidateBlockRequester", 2)
	detachRequestBlocks := candidateEngine.Events.BlockRequester.Tick.Hook(func(blockID models.BlockID) {
		p.networkProtocol.RequestBlock(blockID)
	}, event.WithWorkerPool(wp)).Unhook

	// Attach slot commitments to the chain manager and detach as soon as we switch to that engine
	detachProcessCommitment := candidateEngine.Events.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
		p.chainManager.ProcessCandidateCommitment(details.Commitment)
	}, event.WithWorkerPool(candidateEngine.Workers.CreatePool("ProcessCandidateCommitment", 2))).Unhook

	p.Events.MainEngineSwitched.Hook(func(_ *engine.Engine) {
		detachRequestBlocks()
		detachProcessCommitment()
	}, event.WithMaxTriggerCount(1))

	// Add all the blocks from the forking point to the requester since those will not be passed to the engine by the protocol
	candidateEngine.BlockRequester.StartTickers(blockIDs.Slice())

	// Set the engine as the new candidate
	p.activeEngineMutex.Lock()
	oldCandidateEngine := p.candidateEngine
	p.candidateEngine = candidateEngine
	p.activeEngineMutex.Unlock()

	p.Events.CandidateEngineActivated.Trigger(candidateEngine)

	if oldCandidateEngine != nil {
		oldCandidateEngine.Shutdown()
		if err := oldCandidateEngine.RemoveFromFilesystem(); err != nil {
			p.Events.Error.Trigger(errors.Wrap(err, "error cleaning up replaced candidate engine from file system"))
		}
	}
}

func (p *Protocol) Engine() *engine.Engine {
	// This getter is for backwards compatibility, can be refactored out later on
	return p.MainEngineInstance()
}

func (p *Protocol) MainEngineInstance() *engine.Engine {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.mainEngine
}

func (p *Protocol) CandidateEngineInstance() (instance *engine.Engine) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.candidateEngine
}

func (p *Protocol) ChainManager() (instance *chainmanager.Manager) {
	return p.chainManager
}

func (p *Protocol) linkTo(engineInstance *engine.Engine) {
	p.TipManager.LinkTo(engineInstance)
	p.CongestionControl.LinkTo(engineInstance)
}

func (p *Protocol) Network() *network.Protocol {
	return p.networkProtocol
}

func (p *Protocol) SlotTimeProvider() *slot.TimeProvider {
	return p.Engine().SlotTimeProvider()
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

func WithLedgerProvider(optsLedgerProvider module.Provider[*engine.Engine, ledger.Ledger]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsLedgerProvider = optsLedgerProvider
	}
}

func WithFilterProvider(optsFilterProvider module.Provider[*engine.Engine, filter.Filter]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsFilterProvider = optsFilterProvider
	}
}

func WithSybilProtectionProvider(sybilProtectionProvider module.Provider[*engine.Engine, sybilprotection.SybilProtection]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsSybilProtectionProvider = sybilProtectionProvider
	}
}

func WithThroughputQuotaProvider(throughputQuotaProvider module.Provider[*engine.Engine, throughputquota.ThroughputQuota]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsThroughputQuotaProvider = throughputQuotaProvider
	}
}

func WithNotarizationProvider(notarizationProvider module.Provider[*engine.Engine, notarization.Notarization]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsNotarizationProvider = notarizationProvider
	}
}

func WithTangleProvider(tangleProvider module.Provider[*engine.Engine, tangle.Tangle]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsTangleProvider = tangleProvider
	}
}

func WithConsensusProvider(consensusProvider module.Provider[*engine.Engine, consensus.Consensus]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsConsensusProvider = consensusProvider
	}
}

func WithCongestionControlOptions(opts ...options.Option[congestioncontrol.CongestionControl]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsCongestionControlOptions = append(p.optsCongestionControlOptions, opts...)
	}
}

func WithTipManagerOptions(opts ...options.Option[tipmanager.TipManager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsTipManagerOptions = append(p.optsTipManagerOptions, opts...)
	}
}

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsEngineOptions = append(p.optsEngineOptions, opts...)
	}
}

func WithChainManagerOptions(opts ...options.Option[chainmanager.Manager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsChainManagerOptions = append(p.optsChainManagerOptions, opts...)
	}
}

func WithStorageDatabaseManagerOptions(opts ...options.Option[database.Manager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsStorageDatabaseManagerOptions = append(p.optsStorageDatabaseManagerOptions, opts...)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
