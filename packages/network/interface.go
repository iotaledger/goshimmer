package network

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Interface interface {
	Events() *Events

	Chain() ChainProtocol

	SendBlock(block *models.Block, peers ...*peer.Peer)

	RequestBlock(id models.BlockID, peers ...*peer.Peer)
}

type ChainProtocol interface {
	Events() ChainProtocolEvents

	SendEpochCommitment(commitment *commitment.Commitment, to ...identity.ID)

	RequestEpochCommitment(commitmentID commitment.ID, to ...identity.ID)
}

type ChainProtocolEvents interface {
}
