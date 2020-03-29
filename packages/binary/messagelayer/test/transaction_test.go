package test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/identity"

	"github.com/panjf2000/ants/v2"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func BenchmarkVerifyDataTransactions(b *testing.B) {
	var pool async.WorkerPool
	pool.Tune(runtime.NumCPU() * 2)

	localIdentity := identity.GenerateLocalIdentity()

	transactions := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		tx := message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("some data")))

		transactions[i] = tx.Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		currentIndex := i
		pool.Submit(func() {
			if tx, err, _ := message.FromBytes(transactions[currentIndex]); err != nil {
				b.Error(err)
			} else {
				tx.VerifySignature()
			}
		})
	}

	pool.Shutdown()
}

func BenchmarkVerifySignature(b *testing.B) {
	pool, _ := ants.NewPool(80, ants.WithNonblocking(false))

	localIdentity := identity.GenerateLocalIdentity()

	transactions := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		transactions[i] = message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("test")))
		transactions[i].Bytes()
	}

	var wg sync.WaitGroup

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(1)

		currentIndex := i
		if err := pool.Submit(func() {
			transactions[currentIndex].VerifySignature()

			wg.Done()
		}); err != nil {
			b.Error(err)

			return
		}
	}

	wg.Wait()
}
