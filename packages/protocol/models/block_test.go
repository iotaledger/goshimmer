package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Benchmark(b *testing.B) {
	myBlock := NewBlock(
		WithStrongParents(NewBlockIDs(EmptyBlockID)),
	)
	bytes, err := myBlock.Bytes()
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		restoredBlock := new(Block)
		restoredBlock.FromBytes(bytes)
	}
}
