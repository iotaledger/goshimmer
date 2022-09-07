package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification"
	"github.com/iotaledger/goshimmer/packages/protocol/sybilprotection"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events *Events

	Network             *network.Network
	DatabaseManager     *database.Manager
	NotarizationManager *notarization.Manager
	SnapshotManager     *snapshot.Manager
	EvictionManager     *eviction.Manager[models.BlockID]
	Engine              *engine.Engine
	Solidification      *solidification.Solidification
	SybilProtection     *sybilprotection.SybilProtection

	optsSnapshotFile          string
	optsEngineOptions         []options.Option[engine.Engine]
	optsDBManagerOptions      []options.Option[database.Manager]
	optsSolidificationOptions []options.Option[solidification.Solidification]
}

func New(network *network.Network, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		optsSnapshotFile: "snapshot.bin",
	}, opts, func(p *Protocol) {
		p.Events = NewEvents()
		p.Network = network

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

func (p *Protocol) setupNotarization() {
	// Once an epoch becomes committable, nothing can change anymore. We can safely evict until the given epoch index.
	p.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		p.EvictionManager.EvictUntilEpoch(event.EI)
	}))
}

func (p *Protocol) Start() {
	// configure flow of incoming blocks
	// p.Network.GossipMgr.Events.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
	// 	p.Engine.Tangle.ProcessGossipBlock(event.Data, event.Peer)
	// }))
	//
	// // configure flow of outgoing blocks (gossip upon dispatched blocks)
	// p.Events.Engine.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *tangleold.BlockScheduledEvent) {
	// 	deps.Tangle.Storage.Block(event.BlockID).Consume(func(block *tangleold.Block) {
	// 		deps.GossipMgr.SendBlock(lo.PanicOnErr(block.Bytes()))
	// 	})
	// }))
	//
	// // request missing blocks
	// deps.Tangle.Requester.Events.RequestIssued.Attach(event.NewClosure(func(event *tangleold.RequestIssuedEvent) {
	// 	id := event.BlockID
	// 	Plugin.LogDebugf("requesting missing Block with %s", id)
	//
	// 	deps.GossipMgr.RequestBlock(id.Bytes())
	// }))
}

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
