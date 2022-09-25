package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	network       network.Interface
	requester     *Requester
	optsRequester []options.Option[Requester]
}

func New(network network.Interface, log *logger.Logger, opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(&Solidification{
		network: network,
	}, opts, func(s *Solidification) {
		s.network = network
		s.requester = NewRequester(eviction.NewManager[models.BlockID](), s.optsRequester...)

		s.requester.Events.BlockRequested.Attach(event.NewClosure(func(blockID models.BlockID) {
			s.network.RequestBlock(blockID)
		}))
	})
}

func (s *Solidification) RequestBlock(block *blockdag.Block) {
	s.requester.StartRequest(block)
}

func (s *Solidification) CancelBlockRequest(block *blockdag.Block) {
	s.requester.StopRequest(block)
}

func (s *Solidification) Shutdown() {
	s.requester.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[Requester]) options.Option[Solidification] {
	return func(s *Solidification) {
		s.optsRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
