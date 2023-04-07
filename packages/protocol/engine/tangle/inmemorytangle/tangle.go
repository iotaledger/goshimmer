package inmemorytangle

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag/inmemoryblockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a conflict free replicated data type that allows users to issue their own Blocks with each Block casting
// virtual votes on existing conflicts.
type Tangle struct {
	events *tangle.Events

	blockDAG blockdag.BlockDAG
	booker   booker.Booker

	optsBlockDAGProvider module.Provider[*engine.Engine, blockdag.BlockDAG]
	optsBookerProvider   module.Provider[*engine.Engine, booker.Booker]

	module.Module
}

func NewProvider(opts ...options.Option[Tangle]) module.Provider[*engine.Engine, tangle.Tangle] {
	return module.Provide(func(e *engine.Engine) tangle.Tangle {
		return options.Apply(&Tangle{
			events: tangle.NewEvents(),

			optsBlockDAGProvider: inmemoryblockdag.NewProvider(),
			optsBookerProvider: markerbooker.NewProvider(
				markerbooker.WithSlotCutoffCallback(e.LastConfirmedSlot),
				markerbooker.WithSequenceCutoffCallback(e.FirstUnacceptedMarker),
			),
		},
			opts,
			func(t *Tangle) {
				t.blockDAG = t.optsBlockDAGProvider(e)
				t.booker = t.optsBookerProvider(e)

				t.events.BlockDAG.LinkTo(t.blockDAG.Events())
				t.events.Booker.LinkTo(t.booker.Events())

				e.HookConstructed(func() {
					e.Events.Tangle.LinkTo(t.events)
					t.TriggerInitialized()
				})
			},
			(*Tangle).TriggerConstructed,
		)
	})
}

var _ tangle.Tangle = new(Tangle)

func (t *Tangle) Events() *tangle.Events {
	return t.events
}

func (t *Tangle) Booker() booker.Booker {
	return t.booker
}

func (t *Tangle) BlockDAG() blockdag.BlockDAG {
	return t.blockDAG
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithBlockDAGProvider returns an Option for the Tangle that allows to pass in a provider for the BlockDAG.
func WithBlockDAGProvider(provider module.Provider[*engine.Engine, blockdag.BlockDAG]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsBlockDAGProvider = provider
	}
}

// WithBookerProvider returns an Option for the Tangle that allows to pass in a provider for the Booker.
func WithBookerProvider(provider module.Provider[*engine.Engine, booker.Booker]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsBookerProvider = provider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
