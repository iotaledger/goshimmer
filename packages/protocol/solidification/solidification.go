package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/protocol/chain"
	"github.com/iotaledger/goshimmer/packages/protocol/dispatcher"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification/warpsync"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	network  *network.Network
	parser   *dispatcher.Dispatcher
	protocol *chain.Chain

	requester *requester.Requester
	warpSync  *warpsync.Manager

	WarpSyncMgr   *warpsync.Manager
	gossipManager *gossip.Manager

	optsRequester []options.Option[requester.Requester]
}

func New(network *network.Network, parser *dispatcher.Dispatcher, protocol *chain.Chain, opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(new(Solidification), opts, func(s *Solidification) {
		s.network = network
		s.parser = parser
		s.protocol = protocol

		s.requester = requester.New(protocol.EvictionManager, s.optsRequester...)
		s.warpSync = warpsync.NewManager(protocol.Block, nil, nil)

		// network.WarpSyncMgr.WarpRange()

		protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(s.requester.StartRequest))
		protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(s.requester.StopRequest))
		network.Events.Gossip.BlockRequestReceived.Attach(event.NewClosure(s.onGossipBlockRequestReceived))
		network.Events.Gossip.BlockReceived.Attach(event.NewClosure(s.onGossipBlockReceived))
	})
}

func (s *Solidification) Shutdown() {
	s.requester.Shutdown()
}

func (s *Solidification) onGossipBlockReceived(event *gossip.BlockReceivedEvent) {
	block, err := s.parser.ParseBlock(event.Neighbor, event.Data)
	if err != nil {
		s.protocol.ReportInvalidBlock(event.Neighbor)
	}

	s.protocol.ProcessBlockFromPeer(block, event.Neighbor)
}

func (s *Solidification) onGossipBlockRequestReceived(event *gossip.BlockRequestReceived) {
	block, exists := s.protocol.Block(event.BlockID)
	if !exists {
		return
	}

	s.network.SendBlock(block, event.Neighbor.Peer)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRequesterOptions(opts ...options.Option[requester.Requester]) options.Option[Solidification] {
	return func(s *Solidification) {
		s.optsRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
