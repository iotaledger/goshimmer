package protocol

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/gossip"
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
	EvictionManager     *eviction.LockableManager[models.BlockID]
	Engine              *engine.Engine
	Solidification      *solidification.Solidification
	SybilProtection     *sybilprotection.SybilProtection

	logger *logger.Logger

	optsEngineOptions         []options.Option[engine.Engine]
	optsDBManagerOptions      []options.Option[database.Manager]
	optsSolidificationOptions []options.Option[solidification.Solidification]
}

func New(network *network.Network, logger *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(new(Protocol), opts, func(p *Protocol) {
		p.Events = NewEvents()
		p.Network = network

		p.logger = logger

		var genesisTime time.Time

		p.EvictionManager = eviction.NewManager(models.IsEmptyBlockID).Lockable()
		p.DatabaseManager = database.NewManager(p.optsDBManagerOptions...)
		p.SybilProtection = sybilprotection.New()

		p.Solidification = solidification.New(p.EvictionManager.Manager, p.optsSolidificationOptions...)

		// TODO: when engine is ready
		p.Engine = engine.New(genesisTime, ledger.New(), p.EvictionManager.Manager, p.SybilProtection.ValidatorSet, p.optsEngineOptions...)
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

func (p *Protocol) Start() {
	p.Network.GossipMgr.Events.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
		p.Inbox.PostBlock(event.Data, event.Peer)
	}))

	// TODO: add CongestionControl events
	// configure flow of outgoing blocks (gossip upon dispatched blocks)
	// p.Events.Engine.CongestionControl.Events.BlockScheduled.Attach(event.NewClosure(func(block *models.Block) {
	// 	p.Network.GossipMgr.SendBlock(lo.PanicOnErr(block.Bytes()))
	// }))

	// request missing blocks
	p.Solidification.Requester.Events.BlockRequested.Attach(event.NewClosure(func(id models.BlockID) {
		p.Network.GossipMgr.RequestBlock(lo.PanicOnErr(id.Bytes()))
	}))
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
