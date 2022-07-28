package tangle

import (
	"github.com/iotaledger/hive.go/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// WithDBManagerPath sets the root path to the database manager where all the buckets are stored.
func WithDBManagerPath(path string) options.Option[Tangle] {
	return func(t *Tangle) {
		t.dbManagerPath = path
	}
}

// WithGenesisBlockProvider sets the function that determines whether a block is a solid entrypoint.
func WithGenesisBlockProvider(isSolidEntryPoint func(models.BlockID) *Block) options.Option[Tangle] {
	return func(t *Tangle) {
		t.genesisBlock = isSolidEntryPoint
	}
}

var genesisMetadata = NewBlock(models.NewEmptyBlock(models.EmptyBlockID), WithSolid(true))

// defaultGenesisBlockProvider is a default function that determines whether a block is a solid entrypoint.
func defaultGenesisBlockProvider(blockID models.BlockID) (block *Block) {
	if blockID != models.EmptyBlockID {
		return
	}

	return genesisMetadata
}
