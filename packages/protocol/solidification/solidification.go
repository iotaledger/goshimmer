package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	network             *network.Protocol
	blockRequester      *eventticker.EventTicker[models.BlockID]
	commitmentRequester *eventticker.EventTicker[commitment.ID]

	optsBlockRequester      []options.Option[eventticker.EventTicker[models.BlockID]]
	optsCommitmentRequester []options.Option[eventticker.EventTicker[commitment.ID]]
}

func New(network *network.Protocol, opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(&Solidification{
		network: network,
	}, opts, func(s *Solidification) {
		s.blockRequester = eventticker.New(eviction.NewManager[models.BlockID](), s.optsBlockRequester...)
		s.commitmentRequester = eventticker.New(eviction.NewManager[commitment.ID](), s.optsCommitmentRequester...)

		s.blockRequester.Events.Tick.Attach(event.NewClosure(func(blockID models.BlockID) {
			s.network.RequestBlock(blockID)
		}))

		s.commitmentRequester.Events.Tick.Attach(event.NewClosure(func(commitmentID commitment.ID) {
			s.network.RequestCommitment(commitmentID)
		}))
	})
}

func (s *Solidification) RequestBlock(block *blockdag.Block) {
	s.blockRequester.StartTicker(block.ID())
}

func (s *Solidification) RequestCommitment(id commitment.ID) {
	s.commitmentRequester.StartTicker(id)
}

func (s *Solidification) CancelBlockRequest(block *blockdag.Block) {
	s.blockRequester.StopTicker(block.ID())
}

func (s *Solidification) Shutdown() {
	s.blockRequester.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[eventticker.EventTicker[models.BlockID]]) options.Option[Solidification] {
	return func(s *Solidification) {
		s.optsBlockRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
