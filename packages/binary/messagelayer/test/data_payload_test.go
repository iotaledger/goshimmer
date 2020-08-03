package test

import (
	"runtime"
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/panjf2000/ants/v2"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func BenchmarkVerifyDataMessages(b *testing.B) {
	var pool async.WorkerPool
	pool.Tune(runtime.GOMAXPROCS(0))

	factory := messagefactory.New(mapdb.NewMapDB(), []byte(messagelayer.DBSequenceNumber), identity.GenerateLocalIdentity(), messagefactory.TipSelectorFunc(func() (message.ID, message.ID) { return message.EmptyID, message.EmptyID }))

	messages := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		msg, err := factory.IssuePayload(payload.NewData([]byte("some data")))
		require.NoError(b, err)
		messages[i] = msg.Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		currentIndex := i
		pool.Submit(func() {
			if msg, _, err := message.FromBytes(messages[currentIndex]); err != nil {
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

	factory := messagefactory.New(mapdb.NewMapDB(), []byte(messagelayer.DBSequenceNumber), identity.GenerateLocalIdentity(), messagefactory.TipSelectorFunc(func() (message.ID, message.ID) { return message.EmptyID, message.EmptyID }))

	messages := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		msg, err := factory.IssuePayload(payload.NewData([]byte("some data")))
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
