package parser

import (
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Parser struct{}

func (p *Parser) ParseBlock(neighbor *p2p.Neighbor, bytes []byte) (block *models.Block, err error) {
	block = new(models.Block)
	_, err = block.FromBytes(bytes)
	return
}
