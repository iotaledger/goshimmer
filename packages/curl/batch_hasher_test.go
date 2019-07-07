package curl

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ternary"
)

func BenchmarkBatchHasher_Hash(b *testing.B) {
	batchHasher := NewBatchHasher(243, 81)
	tritsToHash := ternary.Trytes("A999999FF").ToTrits()

	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			batchHasher.Hash(tritsToHash)

			wg.Done()
		}()
	}
	wg.Wait()
}
