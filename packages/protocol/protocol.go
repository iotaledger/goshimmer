package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification"
)

type Protocol struct {
	Events *Events

	Solidification      *solidification.Solidification
	NotarizationManager bool
	DatabaseManager     *database.Manager
	Engine              *engine.Engine
	Network             bool

	optsSolidification []options.Option[solidification.Solidification]
}

func New(opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(new(Protocol), opts, func(p *Protocol) {
		p.Events = NewEvents()
		p.Events.Engine.LinkTo(p.Engine.Events)

		p.Solidification = solidification.New(p.Events.Engine.Tangle.BlockDAG.EvictionManager, p.optsSolidification...)
		p.DatabaseManager = database.New()
		p.Engine = engine.New()
		p.Events.SwitchedEngine.Trigger(p.Engine)

		p.setup()
	})
}

func (e *Protocol) setup() {
	e.Engine.Tangle.BlockDAG.Events.BlockMissing.Hook(event.NewClosure(e.Solidification.Requester.StartRequest))
	e.Engine.Tangle.BlockDAG.Events.MissingBlockAttached.Hook(event.NewClosure(e.Solidification.Requester.StopRequest))
}

func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsSolidification = opts
	}
}
