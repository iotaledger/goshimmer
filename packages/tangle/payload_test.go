package tangle

import (
	"runtime"
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/panjf2000/ants/v2"
)

func BenchmarkVerifyDataMessages(b *testing.B) {
	var pool async.WorkerPool
	pool.Tune(runtime.GOMAXPROCS(0))

	factory := NewMessageFactory(mapdb.NewMapDB(), []byte(DBSequenceNumber), identity.GenerateLocalIdentity(), TipSelectorFunc(func(count int) []MessageID { return []MessageID{EmptyMessageID} }))

	messages := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		msg, err := factory.IssuePayload(payload.NewGenericDataPayload([]byte("some data")))
		require.NoError(b, err)
		messages[i] = msg.Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		currentIndex := i
		pool.Submit(func() {
			if msg, _, err := MessageFromBytes(messages[currentIndex]); err != nil {
				b.Error(err)
			} else {
				msg.VerifySignature()
			}
		})
	}

	pool.Shutdown()
}

func BenchmarkVerifySignature(b *testing.B) {
	pool, _ := ants.NewPool(80, ants.WithNonblocking(false))

	factory := NewMessageFactory(mapdb.NewMapDB(), []byte(DBSequenceNumber), identity.GenerateLocalIdentity(), TipSelectorFunc(func(count int) []MessageID { return []MessageID{EmptyMessageID} }))

	messages := make([]*Message, b.N)
	for i := 0; i < b.N; i++ {
		msg, err := factory.IssuePayload(payload.NewGenericDataPayload([]byte("some data")))
		require.NoError(b, err)
		messages[i] = msg
		messages[i].Bytes()
	}
	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		currentIndex := i
		if err := pool.Submit(func() {
			messages[currentIndex].VerifySignature()
			wg.Done()
		}); err != nil {
			b.Error(err)
			return
		}
	}

	wg.Wait()
}
