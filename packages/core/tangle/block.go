package tangle

import "github.com/iotaledger/goshimmer/packages/core/epoch"

type Block struct{}

func (b *Block) ID() BlockID {
	return 0
}

type BlockMetadata struct {
	id                   BlockID
	missing              bool
	strongParents        []*BlockMetadata
	weakParents          []*BlockMetadata
	likedInsteadParents  []*BlockMetadata
	strongChildren       []*BlockMetadata
	weakChildren         []*BlockMetadata
	likedInsteadChildren []*BlockMetadata
}

func NewBlockMetadata() *BlockMetadata {
	return &BlockMetadata{}
}

func (b *BlockMetadata) ID() BlockID {

}

func NewBlockMetadata(block *Block) *BlockMetadata {
	return &BlockMetadata{}
}

type BlockID int

func (b BlockID) Epoch() epoch.Index {
	return 0
}
