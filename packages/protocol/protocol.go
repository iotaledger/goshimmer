package protocol

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
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/sybilprotection"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events              *Events
	Inbox               *inbox.Inbox
	BlockStorage        *database.PersistentEpochStorage[models.BlockID, models.Block, *models.Block]
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

func New(databaseManager *database.Manager, logger *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events:       NewEvents(),
		ValidatorSet: validator.NewSet(),

		optsSnapshotFile: "snapshot.bin",
	}, opts, func(p *Protocol) {
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

func (p *Protocol) ProcessBlockFromPeer(block *models.Block, neighbor *p2p.Neighbor) {
	p.Inbox.ProcessReceivedBlock(block, neighbor)
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

func (p *Protocol) ReportInvalidBlock(neighbor *p2p.Neighbor) {
	// TODO: increase euristic counter / trigger event for metrics
}

func (p *Protocol) setupNotarization() {
	// Once an epoch becomes committable, nothing can change anymore. We can safely evict until the given epoch index.
	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		p.EvictionManager.EvictUntilEpoch(event.EI)
	}))
}

func (p *Protocol) Start() {
	// p.Events.Engine.CongestionControl.Events.BlockScheduled.Attach(event.NewClosure(p.Network.SendBlock))
	// p.Solidification.Requester.Events.BlockRequested.Attach(event.NewClosure(p.Network.RequestBlock))
}

func emptyActivityConsumer(logs epoch.SnapshotEpochActivity) {}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsEngineOptions = opts
	}
}

func WithSnapshotFile(snapshotFile string) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsSnapshotFile = snapshotFile
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
