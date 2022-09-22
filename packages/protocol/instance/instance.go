package instance

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Instance /////////////////////////////////////////////////////////////////////////////////////////////////////

type Instance struct {
	Events              *Events
	GenesisCommitment   *commitment.Commitment
	BlockStorage        *database.PersistentEpochStorage[models.BlockID, models.Block, *models.BlockID, *models.Block]
	Inbox               *inbox.Inbox
	NotarizationManager *notarization.Manager
	SnapshotManager     *snapshot.Manager
	EvictionManager     *eviction.Manager[models.BlockID]
	Engine              *engine.Engine
	SybilProtection     *sybilprotection.SybilProtection
	ValidatorSet        *validator.Set

	chainDirectory       string
	dbManager            *database.Manager
	logger               *logger.Logger
	optsSnapshotFile     string
	optsSnapshotDepth    int
	optsEngineOptions    []options.Option[engine.Engine]
	optsDBManagerOptions []options.Option[database.Manager]
}

func New(chainDirectory string, logger *logger.Logger, opts ...options.Option[Instance]) (protocol *Instance) {
	return options.Apply(
		&Instance{
			Events:          NewEvents(),
			ValidatorSet:    validator.NewSet(),
			EvictionManager: eviction.NewManager[models.BlockID](),

			chainDirectory:    chainDirectory,
			dbManager:         database.NewManager(database.WithBaseDir(chainDirectory)),
			logger:            logger,
			optsSnapshotFile:  "snapshot.bin",
			optsSnapshotDepth: 5,
		}, opts,

		(*Instance).initBlockStorage,
		(*Instance).initEngine,
		(*Instance).initSybilProtection,
		(*Instance).initNotarizationManager,
		(*Instance).initSnapshotManager,
		(*Instance).loadSnapshot,
		(*Instance).initEvictionManager,
	)
}

func (p *Instance) initBlockStorage() {
	p.BlockStorage = database.NewPersistentEpochStorage[models.BlockID, models.Block](p.dbManager, kvstore.Realm{0x09})
}

func (p *Instance) initEngine() {
	p.Engine = engine.New(time.Time{}, ledger.New(), p.EvictionManager, p.ValidatorSet, p.optsEngineOptions...)

	p.Events.Engine.LinkTo(p.Engine.Events)
}

func (p *Instance) initSybilProtection() {
	p.SybilProtection = sybilprotection.New(p.Engine, p.ValidatorSet)
}

func (p *Instance) initNotarizationManager() {
	p.NotarizationManager = notarization.NewManager(
		notarization.NewEpochCommitmentFactory(p.dbManager.PermanentStorage(), p.optsSnapshotDepth),
		p.Engine,
		notarization.MinCommittableEpochAge(1*time.Minute),
		notarization.BootstrapWindow(2*time.Minute),
		notarization.ManaEpochDelay(2),
	)
}

func (p *Instance) initSnapshotManager() {
	p.SnapshotManager = snapshot.NewManager(p.NotarizationManager, 5)

	p.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		p.SnapshotManager.RemoveSolidEntryPoint(block.ModelsBlock)
	}))

	p.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		block.ForEachParentByType(models.StrongParentType, func(parent models.BlockID) bool {
			if parent.EpochIndex < block.ID().EpochIndex {
				p.SnapshotManager.InsertSolidEntryPoint(parent)
			}

			return true
		})
	}))

	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(e *notarization.EpochCommittableEvent) {
		p.SnapshotManager.AdvanceSolidEntryPoints(e.EI)
	}))
}

func (p *Instance) loadSnapshot() {
	if err := snapshot.LoadSnapshot(
		diskutil.New(p.chainDirectory).Path("snapshot.bin"),
		func(header *ledger.SnapshotHeader) {
			p.GenesisCommitment = header.LatestECRecord

			p.NotarizationManager.LoadECandEIs(header)
		},
		p.SnapshotManager.LoadSolidEntryPoints,
		p.NotarizationManager.LoadOutputsWithMetadata,
		p.NotarizationManager.LoadEpochDiff,
		emptyActivityConsumer,
	); err != nil {
		panic(err)
	}
}

func (p *Instance) initEvictionManager() {
	latestConfirmedIndex, err := p.NotarizationManager.LatestConfirmedEpochIndex()
	if err != nil {
		panic(err)
	}

	p.EvictionManager.EvictUntil(latestConfirmedIndex, p.SnapshotManager.SolidEntryPoints(latestConfirmedIndex))

	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		p.EvictionManager.EvictUntil(event.EI, p.SnapshotManager.SolidEntryPoints(event.EI))
	}))

	// TODO: SET CLOCK
}

func (p *Instance) ProcessBlockFromPeer(block *models.Block, neighbor *p2p.Neighbor) {
	p.Inbox.ProcessReceivedBlock(block, neighbor)
}

func (p *Instance) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := p.Engine.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.ModelsBlock, true
	}

	if id.Index() > p.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return p.BlockStorage.Get(id)
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
