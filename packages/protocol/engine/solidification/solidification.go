package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/solidification/requester"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	*requester.Requester

	blockDAG *blockdag.BlockDAG

	optsRequester []options.Option[requester.Requester]
}

func New(blockDAG *blockdag.BlockDAG, evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(new(Solidification), opts, func(s *Solidification) {
		s.Requester = requester.New(evictionManager, s.optsRequester...)
		s.blockDAG = blockDAG
	}, (*Solidification).setup)
}

func (s *Solidification) Shutdown() {
	s.Requester.Shutdown()
}

func (s *Solidification) setup() {
	s.blockDAG.Events.BlockMissing.Hook(event.NewClosure(s.Requester.StartRequest))
	s.blockDAG.Events.MissingBlockAttached.Hook(event.NewClosure(s.Requester.StopRequest))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[requester.Requester]) options.Option[Solidification] {
	return func(s *Solidification) {
		s.optsRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
