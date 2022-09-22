package network

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Interface interface {
	Events() *Events

	SendBlock(block *models.Block, peers ...*peer.Peer)

	RequestBlock(id models.BlockID, peers ...*peer.Peer)
}
