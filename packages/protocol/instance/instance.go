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
	"github.com/iotaledger/goshimmer/packages/protocol/instance/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tipmanager"
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
	Clock               *clock.Clock
	TipManager          *tipmanager.TipManager
	SybilProtection     *sybilprotection.SybilProtection
	ValidatorSet        *validator.Set

	chainDirectory            string
	dbManager                 *database.Manager
	logger                    *logger.Logger
	optsBootstrappedThreshold time.Duration
	optsSnapshotFile          string
	optsSnapshotDepth         int
	optsEngineOptions         []options.Option[engine.Engine]
	optsDBManagerOptions      []options.Option[database.Manager]
}

func New(chainDirectory string, logger *logger.Logger, opts ...options.Option[Instance]) (protocol *Instance) {
	return options.Apply(
		&Instance{
			Clock:           clock.New(),
			Events:          NewEvents(),
			ValidatorSet:    validator.NewSet(),
			EvictionManager: eviction.NewManager[models.BlockID](),

			chainDirectory:            chainDirectory,
			dbManager:                 database.NewManager(database.WithBaseDir(chainDirectory)),
			logger:                    logger,
			optsBootstrappedThreshold: 10 * time.Second,
			optsSnapshotFile:          "snapshot.bin",
			optsSnapshotDepth:         5,
		}, opts,
		(*Instance).initEngine,
		(*Instance).initClock,
		(*Instance).initBlockStorage,
		(*Instance).initNotarizationManager,
		(*Instance).initSnapshotManager,
		(*Instance).loadSnapshot,
		(*Instance).initEvictionManager,
		(*Instance).initSybilProtection,
	)
}

func (i *Instance) IsBootstrapped() (isBootstrapped bool) {
	return time.Since(i.Clock.RelativeConfirmedTime()) < i.optsBootstrappedThreshold
}

func (i *Instance) initClock() {
	i.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		i.Clock.SetAcceptedTime(block.IssuingTime())
	}))

	i.Events.Clock = i.Clock.Events
}

func (i *Instance) initEngine() {
	i.Engine = engine.New(i.IsBootstrapped, ledger.New(), i.EvictionManager, i.ValidatorSet, i.optsEngineOptions...)

	i.Events.Engine = i.Engine.Events
}

func (i *Instance) initBlockStorage() {
	i.BlockStorage = database.NewPersistentEpochStorage[models.BlockID, models.Block](i.dbManager, kvstore.Realm{0x09})

	i.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		i.BlockStorage.Set(block.ID(), block.ModelsBlock)
	}))

	i.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		i.BlockStorage.Delete(block.ID())
	}))
}

func (i *Instance) initNotarizationManager() {
	i.NotarizationManager = notarization.NewManager(
		i.Clock,
		i.Engine,
		notarization.NewEpochCommitmentFactory(i.dbManager.PermanentStorage(), i.optsSnapshotDepth),
		notarization.MinCommittableEpochAge(1*time.Minute),
		notarization.BootstrapWindow(2*time.Minute),
		notarization.ManaEpochDelay(2),
	)
}

func (i *Instance) initSnapshotManager() {
	i.SnapshotManager = snapshot.NewManager(i.NotarizationManager, 5)

	i.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		i.SnapshotManager.RemoveSolidEntryPoint(block.ModelsBlock)
	}))

	i.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		block.ForEachParentByType(models.StrongParentType, func(parent models.BlockID) bool {
			if parent.EpochIndex < block.ID().EpochIndex {
				i.SnapshotManager.InsertSolidEntryPoint(parent)
			}

			return true
		})
	}))

	i.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(e *notarization.EpochCommittableEvent) {
		i.SnapshotManager.AdvanceSolidEntryPoints(e.EI)
	}))
}

func (i *Instance) loadSnapshot() {
	if err := snapshot.LoadSnapshot(
		diskutil.New(i.chainDirectory).Path("snapshot.bin"),
		func(header *ledger.SnapshotHeader) {
			i.GenesisCommitment = header.LatestECRecord

			i.NotarizationManager.LoadECandEIs(header)
		},
		i.SnapshotManager.LoadSolidEntryPoints,
		func(outputsWithMetadata []*ledger.OutputWithMetadata) {
			i.NotarizationManager.LoadOutputsWithMetadata(outputsWithMetadata)

			i.Engine.Ledger.LoadOutputsWithMetadata(outputsWithMetadata)

			// TODO FILL INDEXER
		},
		func(epochDiffs *ledger.EpochDiff) {
			i.NotarizationManager.LoadEpochDiff(epochDiffs)

			if err := i.Engine.Ledger.LoadEpochDiff(epochDiffs); err != nil {
				panic(err)
			}

			// TODO FILL INDEXER
		},
		emptyActivityConsumer,
	); err != nil {
		panic(err)
	}
}

func (i *Instance) initEvictionManager() {
	latestConfirmedIndex, err := i.NotarizationManager.LatestConfirmedEpochIndex()
	if err != nil {
		panic(err)
	}

	i.EvictionManager.EvictUntil(latestConfirmedIndex, i.SnapshotManager.SolidEntryPoints(latestConfirmedIndex))

	i.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		i.EvictionManager.EvictUntil(event.EI, i.SnapshotManager.SolidEntryPoints(event.EI))
	}))

	i.Clock.SetAcceptedTime(latestConfirmedIndex.EndTime())

	i.Events.EvictionManager = i.EvictionManager.Events

}

func (i *Instance) initTipManager() {
	i.TipManager = tipmanager.New(i.Engine.Tangle, i.Engine.Consensus.Gadget, i.Engine.CongestionControl.Scheduler.Block, i.Clock.AcceptedTime, i.GenesisCommitment.Index().EndTime())

	i.Engine.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(i.TipManager.AddTip))

	i.Engine.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		i.TipManager.RemoveStrongParents(block.ModelsBlock)
	}))

	i.Engine.Events.Tangle.BlockDAG.BlockOrphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		if schedulerBlock, exists := i.Engine.CongestionControl.Scheduler.Block(block.ID()); exists {
			i.TipManager.DeleteTip(schedulerBlock)
		}
	}))

	// TODO: enable once this event is implemented
	// t.tangle.TipManager.Events.AllChildrenOrphaned.Hook(event.NewClosure(func(block *Block) {
	// 	if clock.Since(block.IssuingTime()) > tipLifeGracePeriod {
	// 		return
	// 	}
	//
	// 	t.addTip(block)
	// }))

	i.Events.TipManager = i.TipManager.Events
}

func (i *Instance) initSybilProtection() {
	i.SybilProtection = sybilprotection.New(i.ValidatorSet, i.Clock.RelativeAcceptedTime)

	i.Engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(i.SybilProtection.TrackActiveValidators))
}

func (i *Instance) ProcessBlockFromPeer(block *models.Block, neighbor *p2p.Neighbor) {
	i.Inbox.ProcessReceivedBlock(block, neighbor)
}

func (i *Instance) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := i.Engine.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.ModelsBlock, true
	}

	if id.Index() > i.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return i.BlockStorage.Get(id)
}

func emptyActivityConsumer(logs activitylog.SnapshotEpochActivity) {}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBootstrapThreshold(threshold time.Duration) options.Option[Instance] {
	return func(e *Instance) {
		e.optsBootstrappedThreshold = threshold
	}
}

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
