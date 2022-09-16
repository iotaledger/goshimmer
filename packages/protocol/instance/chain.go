package chain

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Chain ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Chain struct {
	Events              *Events
	Inbox               *inbox.Inbox
	NotarizationManager *notarization.Manager
	SnapshotManager     *snapshot.Manager
	EvictionManager     *eviction.Manager[models.BlockID]
	Engine              *engine.Engine
	SybilProtection     *sybilprotection.SybilProtection
	ValidatorSet        *validator.Set

	logger *logger.Logger

	optsSnapshotFile     string
	optsEngineOptions    []options.Option[engine.Engine]
	optsDBManagerOptions []options.Option[database.Manager]
}

func New(databaseManager *database.Manager, logger *logger.Logger, opts ...options.Option[Chain]) (protocol *Chain) {
	return options.Apply(&Chain{
		Events:       NewEvents(),
		ValidatorSet: validator.NewSet(),

		optsSnapshotFile: "snapshot.bin",
	}, opts, func(p *Chain) {
		p.BlockStorage = database.New[models.BlockID, models.Block, *models.Block](databaseManager, kvstore.Realm{0x09})

		err := snapshot.LoadSnapshot(
			p.optsSnapshotFile,
			p.NotarizationManager.LoadECandEIs,
			p.SnapshotManager.LoadSolidEntryPoints,
			p.NotarizationManager.LoadOutputsWithMetadata,
			p.NotarizationManager.LoadEpochDiff,
			emptyActivityConsumer,
		)
		snapshotIndex, err := p.NotarizationManager.LatestConfirmedEpochIndex()
		if err != nil {
			return
		}
		p.logger = logger

		p.EvictionManager = eviction.NewManager(snapshotIndex, func(index epoch.Index) *set.AdvancedSet[models.BlockID] {
			// TODO: implement me and set snapshot epoch!
			// p.SnapshotManager.GetSolidEntryPoints(index)
			return set.NewAdvancedSet[models.BlockID]()
		})

		// TODO: when engine is ready
		p.Engine = engine.New(snapshotIndex.EndTime(), ledger.New(), p.EvictionManager, p.ValidatorSet, p.optsEngineOptions...)
		p.Events.Engine.LinkTo(p.Engine.Events)

		p.SybilProtection = sybilprotection.New(p.Engine, p.ValidatorSet)
	})
}

func (p *Chain) ProcessBlockFromPeer(block *models.Block, neighbor *p2p.Neighbor) {
	p.Inbox.ProcessReceivedBlock(block, neighbor)
}

func (p *Chain) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := p.Engine.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.Block, true
	}

	if id.Index() > p.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return p.BlockStorage.Get(id)
}

func (p *Chain) ReportInvalidBlock(neighbor *p2p.Neighbor) {
	// TODO: increase euristic counter / trigger event for metrics
}

func (p *Chain) setupNotarization() {
	// Once an epoch becomes committable, nothing can change anymore. We can safely evict until the given epoch index.
	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		p.EvictionManager.EvictUntilEpoch(event.EI)
	}))
}

func (p *Chain) Start() {
	// p.Events.Engine.CongestionControl.Events.BlockScheduled.Attach(event.NewClosure(p.Network.SendBlock))
	// p.Solidification.Requester.Events.BlockRequested.Attach(event.NewClosure(p.Network.RequestBlock))
}

func emptyActivityConsumer(logs epoch.SnapshotEpochActivity) {}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Chain] {
	return func(p *Chain) {
		p.optsEngineOptions = opts
	}
}

func WithSnapshotFile(snapshotFile string) options.Option[Chain] {
	return func(p *Chain) {
		p.optsSnapshotFile = snapshotFile
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
