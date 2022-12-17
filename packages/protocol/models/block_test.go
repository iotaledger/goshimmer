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

	restoredBlock := new(Block)
	for i := 0; i < b.N; i++ {
		_, err := restoredBlock.FromBytes(bytes)
		require.NoError(b, err)
	}
}
