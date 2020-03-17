package test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/async"

	"github.com/panjf2000/ants/v2"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
)

func BenchmarkVerifyDataTransactions(b *testing.B) {
	var pool async.WorkerPool
	pool.Tune(runtime.NumCPU() * 2)

	transactions := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		tx := transaction.New(transaction.EmptyId, transaction.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("some data")))

		if marshaledTransaction, err := tx.MarshalBinary(); err != nil {
			b.Error(err)
		} else {
			transactions[i] = marshaledTransaction
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		currentIndex := i
		pool.Submit(func() {
			if tx, err, _ := transaction.FromBytes(transactions[currentIndex]); err != nil {
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

	transactions := make([]*transaction.Transaction, b.N)
	for i := 0; i < b.N; i++ {
		transactions[i] = transaction.New(transaction.EmptyId, transaction.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("test")))
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
