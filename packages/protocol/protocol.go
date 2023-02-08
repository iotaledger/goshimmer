package protocol

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
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
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
)

const (
	mainBaseDir      = "main"
	candidateBaseDir = "candidate"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events            *Events
	CongestionControl *congestioncontrol.CongestionControl
	TipManager        *tipmanager.TipManager
	chainManager      *chainmanager.Manager

	dispatcher           network.Endpoint
	networkProtocol      *network.Protocol
	directory            *utils.Directory
	activeEngineMutex    sync.RWMutex
	engine               *engine.Engine
	candidateEngine      *engine.Engine
	storage              *storage.Storage
	candidateStorage     *storage.Storage
	mainChain            commitment.ID
	candidateChain       commitment.ID
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
		(*Protocol).initDirectory,
		(*Protocol).initCongestionControl,
		(*Protocol).initMainChainStorage,
		(*Protocol).initMainEngine,
		(*Protocol).initChainManager,
		(*Protocol).initTipManager,
	)
}

// Run runs the protocol.
func (p *Protocol) Run() {
	p.CongestionControl.Run()
	p.linkTo(p.engine)

	if err := p.engine.Initialize(p.optsSnapshotPath); err != nil {
		panic(err)
	}

	p.initNetworkProtocol()
}

// Shutdown shuts down the protocol.
func (p *Protocol) Shutdown() {
	p.CongestionControl.Shutdown()
	p.engine.Shutdown()
	p.storage.Shutdown()

	if p.candidateEngine != nil {
		p.candidateEngine.Shutdown()
	}

	if p.candidateStorage != nil {
		p.candidateStorage.Shutdown()
	}
}

func (p *Protocol) WorkerPools() map[string]*workerpool.UnboundedWorkerPool {
	wps := make(map[string]*workerpool.UnboundedWorkerPool)
	wps["CongestionControl"] = p.CongestionControl.WorkerPool()
	return lo.MergeMaps(wps, p.engine.WorkerPools())
}

func (p *Protocol) initDirectory() {
	p.directory = utils.NewDirectory(p.optsBaseDirectory)
}

func (p *Protocol) initMainChainStorage() {
	p.storage = storage.New(p.directory.Path(mainBaseDir), DatabaseVersion, p.optsStorageDatabaseManagerOptions...)

	p.Events.Engine.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.storage.PruneUntilEpoch(epochIndex - epoch.Index(p.optsPruningThreshold))
	}))
}

func (p *Protocol) initCongestionControl() {
	p.CongestionControl = congestioncontrol.New(p.optsCongestionControlOptions...)

	p.Events.CongestionControl = p.CongestionControl.Events
}

func (p *Protocol) initNetworkProtocol() {
	p.networkProtocol = network.NewProtocol(p.dispatcher)

	p.networkProtocol.Events.BlockRequestReceived.Attach(event.NewClosure(func(event *network.BlockRequestReceivedEvent) {
		if block, exists := p.Engine().Block(event.BlockID); exists {
			p.networkProtocol.SendBlock(block, event.Source)
		}
	}))

	p.networkProtocol.Events.BlockReceived.Attach(event.NewClosure(func(event *network.BlockReceivedEvent) {
		if err := p.ProcessBlock(event.Block, event.Source); err != nil {
			fmt.Print(err)
		}
	}))

	p.networkProtocol.Events.EpochCommitmentReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentReceivedEvent) {
		p.chainManager.ProcessCommitment(event.Commitment)
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
		p.ProcessAttestationsRequest(event.Index, event.Source)
	}))

	p.networkProtocol.Events.AttestationsReceived.Attach(event.NewClosure(func(event *network.AttestationsReceivedEvent) {
		p.ProcessAttestations(event.Source)
	}))
}

func (p *Protocol) initMainEngine() {
	p.engine = engine.New(p.storage, p.optsSybilProtectionProvider, p.optsThroughputQuotaProvider, p.optsEngineOptions...)
}

func (p *Protocol) initChainManager() {
	p.chainManager = chainmanager.NewManager(p.Engine().Storage.Settings.LatestCommitment())

	p.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		p.chainManager.ProcessCommitment(details.Commitment)
	}))

	p.Events.Engine.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		p.chainManager.CommitmentRequester.EvictUntil(epochIndex)
	}))
}

func (p *Protocol) initTipManager() { // TODO: SWITCH ENGINE SIMILAR TO REQUESTER
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
	isSolid, chain, _ := p.chainManager.ProcessCommitment(block.Commitment())
	if !isSolid {
		return errors.Errorf("Chain is not solid: ", block.Commitment().ID(), "\nLatest commitment: ", p.storage.Settings.LatestCommitment().ID(), "\nBlock ID: ", block.ID())
	}

	if mainChain := p.storage.Settings.ChainID(); chain.ForkingPoint.ID() == mainChain {
		p.Engine().ProcessBlockFromPeer(block, src)
		return nil
	}

	if p.Engine().IsBootstrapped() {
		// panic(fmt.Sprintf("different commitment on block %s\nforking at %s\nour latest commitment %s", block, chain.ForkingPoint.ID(), lo.PanicOnErr(p.storage.Commitments.Load(chain.ForkingPoint.ID().Index())).ID()))
		fmt.Printf("different commitment on block %s\nforking at %s\nour latest commitment %s\n", block, chain.ForkingPoint.ID(), lo.PanicOnErr(p.storage.Commitments.Load(chain.ForkingPoint.ID().Index())).ID())
		fmt.Println("pppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppp")
		return nil
	}

	if candidateEngine, candidateStorage := p.CandidateEngine(), p.CandidateStorage(); candidateEngine != nil && candidateStorage != nil {
		if candidateChain := candidateStorage.Settings.ChainID(); chain.ForkingPoint.ID() == candidateChain {
			candidateEngine.ProcessBlockFromPeer(block, src)
			return nil
		}
	}
	return errors.Errorf("block was not processed")
}

func (p *Protocol) ProcessAttestationsRequest(epochIndex epoch.Index, src identity.ID) {
	// p.networkProtocol.SendAttestations(p.Engine().SybilProtection.Attestations(epochIndex), src)
}

func (p *Protocol) ProcessAttestations(src identity.ID) {
	// TODO: process attestations and evluate chain switch!
}

func (p *Protocol) Engine() (instance *engine.Engine) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.engine
}

func (p *Protocol) ChainManager() (instance *chainmanager.Manager) {
	return p.chainManager
}

func (p *Protocol) CandidateEngine() (instance *engine.Engine) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.candidateEngine
}

// MainStorage returns the underlying storage of the main chain.
func (p *Protocol) MainStorage() (mainStorage *storage.Storage) {
	return p.storage
}

func (p *Protocol) CandidateStorage() (chainstorage *storage.Storage) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.candidateStorage
}

func (p *Protocol) linkTo(engine *engine.Engine) {
	p.Events.Engine.LinkTo(engine.Events)
	p.TipManager.LinkTo(engine)
	p.CongestionControl.LinkTo(engine)
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
