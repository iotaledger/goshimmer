package tangle

import "github.com/iotaledger/hive.go/generics/options"

// WithDBManagerPath sets the root path to the database manager where all the buckets are stored.
func WithDBManagerPath(path string) options.Option[Tangle] {
	return func(t *Tangle) {
		t.optsDBManagerPath = path
	}
}

// WithSolidEntryPointFunc sets the function that determines whether a block is a solid entrypoint.
func WithSolidEntryPointFunc(isSolidEntryPoint func(BlockID) bool) options.Option[Tangle] {
	return func(t *Tangle) {
		t.optsIsSolidEntryPoint = isSolidEntryPoint
	}
}

// IsGenesisBlock is a default function that determines whether a block is a solid entrypoint.
func IsGenesisBlock(blockID BlockID) (isGenesisBlock bool) {
	return blockID == EmptyBlockID
}
