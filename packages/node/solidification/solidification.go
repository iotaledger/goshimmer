package solidification

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/node/solidification/requester"
	"github.com/iotaledger/goshimmer/packages/node/solidification/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol"
)

// region Solidification ///////////////////////////////////////////////////////////////////////////////////////////////

type Solidification struct {
	Requester *requester.Requester
	WarpSync  *warpsync.Manager

	optsRequester []options.Option[requester.Requester]
}

func New(protocol *protocol.Protocol, network *network.Network, opts ...options.Option[Solidification]) (solidification *Solidification) {
	return options.Apply(new(Solidification), opts, func(s *Solidification) {
		s.WarpSync = warpsync.NewManager(protocol.Block, nil, nil)
		network.WarpSyncMgr.WarpRange()

		s.Requester = requester.New(protocol.EvictionManager, s.optsRequester...)
		protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(s.Requester.StartRequest))
		protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(s.Requester.StopRequest))
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
