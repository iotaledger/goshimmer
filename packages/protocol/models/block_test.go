package models

import (
	"fmt"
	"runtime"
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

func BenchmarkSerixMemLeak(b *testing.B) {
	const count = 1000000

	PrintMemUsage("start 'FromBytes'")
	{
		var randomBlockID1, randomBlockID2 BlockID
		randomBlockID1.FromRandomness()
		randomBlockID2.FromRandomness()

		myBlock := NewBlock(
			WithStrongParents(NewBlockIDs(EmptyBlockID, randomBlockID2)),
			WithWeakParents(NewBlockIDs(randomBlockID1)),
			WithLikedInsteadParents(NewBlockIDs(randomBlockID2)),
		)
		bytes, err := myBlock.Bytes()
		if err != nil {
			panic(err)
		}
		objects := make([]*Block, 0, count)

		for i := 0; i < count; i++ {
			restoredBlock := new(Block)
			_, err = restoredBlock.FromBytes(bytes)
			if err != nil {
				panic(err)
			}
			objects = append(objects, restoredBlock)
		}

		objects = nil
		PrintMemUsage("before GC")

		// Force GC to clear up, should see a memory drop
		runtime.GC()
		PrintMemUsage("final")
	}

	fmt.Println("========================================================")
	PrintMemUsage("start 'Bytes'")

	{
		var randomBlockID1, randomBlockID2 BlockID
		randomBlockID1.FromRandomness()
		randomBlockID2.FromRandomness()

		myBlock := NewBlock(
			WithStrongParents(NewBlockIDs(EmptyBlockID, randomBlockID2)),
			WithWeakParents(NewBlockIDs(randomBlockID1)),
			WithLikedInsteadParents(NewBlockIDs(randomBlockID2)),
		)

		for i := 0; i < count; i++ {
			_, err := myBlock.Bytes()
			if err != nil {
				panic(err)
			}
		}

		PrintMemUsage("befre GC")

		runtime.GC()
		PrintMemUsage("final")
	}

}

func MemStats() runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}

// PrintMemUsage outputs the current, total and OS memory being used. As well as the number
// of garage collection cycles completed.
func PrintMemUsage(s string) {
	m := MemStats()
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("%s : ", s)
	fmt.Printf("Alloc = %v KiB", bToKb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v KiB", bToKb(m.TotalAlloc))
	fmt.Printf("\tSys = %v KiB", bToKb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
	fmt.Println("-----------------------------------------------------------------------------------------")
}

func bToKb(b uint64) uint64 {
	return b / 1024
}
