package inbox

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Inbox struct {
	Events *Events
}

func New(opts ...options.Option[Inbox]) (inbox *Inbox) {
	return options.Apply(&Inbox{
		Events: NewEvents(),
	}, opts)
}

func (i Inbox) PostBlock(blockData []byte, peer *peer.Peer) {
	block := new(models.Block)
	if _, err := block.FromBytes(blockData); err != nil {
		// do sth (i.e. increase invalid blocks counter for peer)
	}

	i.Events.BlockReceived.Trigger(block)
}
