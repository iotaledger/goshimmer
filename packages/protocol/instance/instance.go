package instance

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Instance /////////////////////////////////////////////////////////////////////////////////////////////////////

type Instance struct {
	Events              *Events
	BlockStorage        *database.PersistentEpochStorage[models.BlockID, models.Block, *models.BlockID, *models.Block]
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

func New(databaseManager *database.Manager, logger *logger.Logger, opts ...options.Option[Instance]) (protocol *Instance) {
	return options.Apply(&Instance{
		Events:       NewEvents(),
		ValidatorSet: validator.NewSet(),

		optsSnapshotFile: "snapshot.bin",
	}, opts, func(p *Instance) {
		p.BlockStorage = database.New[models.BlockID, models.Block](databaseManager, kvstore.Realm{0x09})

		if err := snapshot.LoadSnapshot(
			p.optsSnapshotFile,
			p.NotarizationManager.LoadECandEIs,
			p.SnapshotManager.LoadSolidEntryPoints,
			p.NotarizationManager.LoadOutputsWithMetadata,
			p.NotarizationManager.LoadEpochDiff,
			emptyActivityConsumer,
		); err != nil {
			panic(err)
		}

		snapshotIndex, err := p.NotarizationManager.LatestConfirmedEpochIndex()
		if err != nil {
			panic(err)
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

func (p *Instance) ProcessBlockFromPeer(block *models.Block, neighbor *p2p.Neighbor) {
	p.Inbox.ProcessReceivedBlock(block, neighbor)
}

func (p *Instance) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := p.Engine.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.Block, true
	}

	if id.Index() > p.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return p.BlockStorage.Get(id)
}

func (p *Instance) ReportInvalidBlock(neighbor *p2p.Neighbor) {
	// TODO: increase euristic counter / trigger event for metrics
}

func (p *Instance) setupNotarization() {
	// Once an epoch becomes committable, nothing can change anymore. We can safely evict until the given epoch index.
	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		p.EvictionManager.EvictUntilEpoch(event.EI)
	}))
}

func (p *Instance) Start() {
	// p.Events.Engine.CongestionControl.Events.BlockScheduled.Attach(event.NewClosure(p.Network.SendBlock))
	// p.Solidification.Requester.Events.BlockRequested.Attach(event.NewClosure(p.Network.RequestBlock))
}

func emptyActivityConsumer(logs activitylog.SnapshotEpochActivity) {}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Instance] {
	return func(p *Instance) {
		p.optsEngineOptions = opts
	}
}

func WithSnapshotFile(snapshotFile string) options.Option[Instance] {
	return func(p *Instance) {
		p.optsSnapshotFile = snapshotFile
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
