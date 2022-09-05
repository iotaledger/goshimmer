package protocol

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
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

	DatabaseManager     *database.Manager
	NotarizationManager *notarization.Manager
	EvictionManager     *eviction.Manager[models.BlockID]
	Engine              *engine.Engine
	Solidification      *solidification.Solidification
	SybilProtection     *sybilprotection.SybilProtection
	Network             bool

	optsEngineOptions         []options.Option[engine.Engine]
	optsDBManagerOptions      []options.Option[database.Manager]
	optsSolidificationOptions []options.Option[solidification.Solidification]
}

func New(opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(new(Protocol), opts, func(p *Protocol) {
		p.Events = NewEvents()

		var genesisTime time.Time

		p.EvictionManager = eviction.NewManager(models.IsEmptyBlockID)
		p.DatabaseManager = database.NewManager("goshimmer" /* p.optsDBManagerOptions... */)
		p.SybilProtection = sybilprotection.New()

		p.Solidification = solidification.New(p.EvictionManager, p.optsSolidificationOptions...)

		// TODO: when engine is ready
		p.Engine = engine.New(genesisTime, ledger.New(), p.EvictionManager, p.SybilProtection.ValidatorSet, p.optsEngineOptions...)
		p.Events.Engine.LinkTo(p.Engine.Events)
	})
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
