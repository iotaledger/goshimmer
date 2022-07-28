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

// WithSolidEntryPointFunc sets the function that determines whether a block is a solid entrypoint.
func WithSolidEntryPointFunc(isSolidEntryPoint func(models.BlockID) bool) options.Option[Tangle] {
	return func(t *Tangle) {
		t.isSolidEntryPoint = isSolidEntryPoint
	}
}

// IsGenesisBlock is a default function that determines whether a block is a solid entrypoint.
func IsGenesisBlock(blockID models.BlockID) (isGenesisBlock bool) {
	return blockID == models.EmptyBlockID
}
