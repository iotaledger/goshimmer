package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification/requester"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	*requester.Requester

	optsRequester []options.Option[requester.Requester]
}

func New(evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(new(Solidification), opts, func(s *Solidification) {
		s.Requester = requester.New(evictionManager, s.optsRequester...)
	})
}

func (s *Solidification) Shutdown() {
	s.Requester.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[requester.Requester]) options.Option[Solidification] {
	return func(s *Solidification) {
		s.optsRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
