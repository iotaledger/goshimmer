package inbox

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Inbox struct {
}

func New(opts ...options.Option[Inbox]) (inbox *Inbox) {
	return options.Apply(&Inbox{}, opts)
}

func (i Inbox) PostBlock(blockData []byte, peer *peer.Peer) {
	block := new(models.Block)
	if err := block.FromBytes(blockData); err != nil {
		// do sth
	}

	p.Heuristic
	p.Engine.Tangle.ProcessGossipBlock(event.Data, event.Peer)
}
