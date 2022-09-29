package requester

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Requester ////////////////////////////////////////////////////////////////////////////////////////////////////

type Requester struct {
	BlockEvictionManager      *eviction.Manager[models.BlockID]
	CommitmentEvictionManager *eviction.Manager[commitment.ID]

	network             *network.Protocol
	blockRequester      *eventticker.EventTicker[models.BlockID]
	commitmentRequester *eventticker.EventTicker[commitment.ID]

	optsBlockRequester      []options.Option[eventticker.EventTicker[models.BlockID]]
	optsCommitmentRequester []options.Option[eventticker.EventTicker[commitment.ID]]
}

func New(network *network.Protocol, opts ...options.Option[Requester]) (solidification *Requester) {
	return options.Apply(&Requester{
		BlockEvictionManager:      eviction.NewManager[models.BlockID](),
		CommitmentEvictionManager: eviction.NewManager[commitment.ID](),

		network: network,
	}, opts, func(s *Requester) {
		s.blockRequester = eventticker.New(s.BlockEvictionManager, s.optsBlockRequester...)
		s.commitmentRequester = eventticker.New(s.CommitmentEvictionManager, s.optsCommitmentRequester...)

		s.blockRequester.Events.Tick.Attach(event.NewClosure(func(blockID models.BlockID) {
			s.network.RequestBlock(blockID)
		}))

		s.commitmentRequester.Events.Tick.Attach(event.NewClosure(func(commitmentID commitment.ID) {
			s.network.RequestCommitment(commitmentID)
		}))
	})
}

func (s *Requester) RequestBlock(id models.BlockID) {
	s.blockRequester.StartTicker(id)
}

func (s *Requester) CancelBlockRequest(id models.BlockID) {
	s.blockRequester.StopTicker(id)
}

func (s *Requester) RequestCommitment(id commitment.ID) {
	s.commitmentRequester.StartTicker(id)
}

func (s *Requester) CancelCommitmentRequest(id commitment.ID) {
	s.commitmentRequester.StopTicker(id)
}

func (s *Requester) Shutdown() {
	s.blockRequester.Shutdown()
	s.commitmentRequester.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[eventticker.EventTicker[models.BlockID]]) options.Option[Requester] {
	return func(s *Requester) {
		s.optsBlockRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
