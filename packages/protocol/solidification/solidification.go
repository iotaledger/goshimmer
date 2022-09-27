package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/requester"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	network             network.Interface
	commitmentRequester *requester.Requester[commitment.ID]
	blockRequester      *requester.Requester[models.BlockID]
	optsRequester       []options.Option[requester.Requester[models.BlockID]]
}

func New(network network.Interface, log *logger.Logger, opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(&Solidification{
		network: network,
	}, opts, func(s *Solidification) {
		s.network = network
		s.blockRequester = requester.NewRequester(eviction.NewManager[models.BlockID](), s.optsRequester...)

		s.blockRequester.Events.Request.Attach(event.NewClosure(func(blockID models.BlockID) {
			s.network.RequestBlock(blockID)
		}))
	})
}

func (s *Solidification) RequestBlock(block *blockdag.Block) {
	s.blockRequester.StartRequest(block.ID())
}

func (s *Solidification) CancelBlockRequest(block *blockdag.Block) {
	s.blockRequester.StopRequest(block.ID())
}

func (s *Solidification) Shutdown() {
	s.blockRequester.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[requester.Requester[models.BlockID]]) options.Option[Solidification] {
	return func(s *Solidification) {
		s.optsRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
