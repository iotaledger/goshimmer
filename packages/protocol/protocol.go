package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification"
	"github.com/iotaledger/goshimmer/packages/protocol/sybilprotection"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events              *Events
	Network             *network.Network
	Inbox               *inbox.Inbox
	DatabaseManager     *database.Manager
	BlockStorage        *database.PersistentEpochStorage[models.BlockID, models.Block, *models.Block]
	NotarizationManager *notarization.Manager
	SnapshotManager     *snapshot.Manager
	EvictionManager     *eviction.Manager[models.BlockID]
	Engine              *engine.Engine
	Solidification      *solidification.Solidification
	SybilProtection     *sybilprotection.SybilProtection

	logger *logger.Logger

	optsSnapshotFile          string
	optsEngineOptions         []options.Option[engine.Engine]
	optsDBManagerOptions      []options.Option[database.Manager]
	optsSolidificationOptions []options.Option[solidification.Solidification]
}

func New(network *network.Network, logger *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events:  NewEvents(),
		Network: network,

		optsSnapshotFile: "snapshot.bin",
	}, opts, func(p *Protocol) {
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
		p.DatabaseManager = database.NewManager(p.optsDBManagerOptions...)
		p.SybilProtection = sybilprotection.New()

		p.Solidification = solidification.New(p.EvictionManager, p.optsSolidificationOptions...)

		// TODO: when engine is ready
		p.Engine = engine.New(snapshotIndex.EndTime(), ledger.New(), p.EvictionManager, p.SybilProtection.ValidatorSet, p.optsEngineOptions...)
		p.Events.Engine.LinkTo(p.Engine.Events)
	})
}

func (p *Protocol) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := p.Engine.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.Block, true
	}

	if id.Index() > p.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return p.BlockStorage.Get(id)
}

func (p *Protocol) setupNotarization() {
	// Once an epoch becomes committable, nothing can change anymore. We can safely evict until the given epoch index.
	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		p.EvictionManager.EvictUntilEpoch(event.EI)
	}))
}

func (p *Protocol) Start() {
	p.Network.Events.BlockReceived.Attach(event.NewClosure(p.Inbox.ProcessBlockReceivedEvent))

	// TODO: add CongestionControl events
	// configure flow of outgoing blocks (gossip upon dispatched blocks)
	// p.Events.Engine.CongestionControl.Events.BlockScheduled.Attach(event.NewClosure(func(block *models.Block) {
	// 	p.Network.GossipMgr.SendBlock(lo.PanicOnErr(block.Bytes()))
	// }))

	// request missing blocks
	p.Solidification.Requester.Events.BlockRequested.Attach(event.NewClosure(p.Network.RequestBlock))
}

func emptyActivityConsumer(logs epoch.SnapshotEpochActivity) {}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithDBManagerOptions(opts ...options.Option[database.Manager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsDBManagerOptions = opts
	}
}

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsEngineOptions = opts
	}
}

func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsSolidificationOptions = opts
	}
}

func WithSnapshotFile(snapshotFile string) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsSnapshotFile = snapshotFile
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
