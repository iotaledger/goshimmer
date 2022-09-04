package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification"
)

type Protocol struct {
	Events *Events

	Solidification      *solidification.Solidification
	NotarizationManager bool
	EvictionManager     *eviction.Manager[models.BlockID]
	DatabaseManager     *database.Manager
	Engine              *engine.Engine
	Network             bool

	optsSolidification []options.Option[solidification.Solidification]
}

func New(opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(new(Protocol), opts, func(p *Protocol) {
		p.EvictionManager = eviction.NewManager(models.IsEmptyBlockID)
		p.DatabaseManager = database.NewManager("goshimmer")

		p.Engine = engine.New(ledger.New(), p.EvictionManager, validator.NewSet())
		p.Solidification = solidification.New(p.EvictionManager, p.optsSolidification...)

		p.Events = NewEvents()
		p.Events.Engine.LinkTo(p.Engine.Events)
	})
}

func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsSolidification = opts
	}
}
