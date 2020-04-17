package tangle

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

func BenchmarkTangle_AttachTransaction(b *testing.B) {
	dir, err := ioutil.TempDir("", b.Name())
	require.NoError(b, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	config.Node.Set(database.CFG_DIRECTORY, dir)

	tangle := New(database.GetBadgerInstance())
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	testIdentity := identity.GenerateLocalIdentity()

	transactionBytes := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		transactionBytes[i] = message.New(message.EmptyId, message.EmptyId, testIdentity, time.Now(), 0, payload.NewData([]byte("some data")))
		transactionBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.AttachMessage(transactionBytes[i])
	}

	tangle.Shutdown()
}

func TestTangle_AttachTransaction(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	config.Node.Set(database.CFG_DIRECTORY, dir)

	messageTangle := New(database.GetBadgerInstance())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Message) {
			fmt.Println("ATTACHED:", transaction.Id())
		})
	}))

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Message) {
			fmt.Println("SOLID:", transaction.Id())
		})
	}))

	messageTangle.Events.MessageUnsolidifiable.Attach(events.NewClosure(func(transactionId message.Id) {
		fmt.Println("UNSOLIDIFIABLE:", transactionId)
	}))

	messageTangle.Events.MessageMissing.Attach(events.NewClosure(func(transactionId message.Id) {
		fmt.Println("MISSING:", transactionId)
	}))

	messageTangle.Events.MessageRemoved.Attach(events.NewClosure(func(transactionId message.Id) {
		fmt.Println("REMOVED:", transactionId)
	}))

	localIdentity1 := identity.GenerateLocalIdentity()
	localIdentity2 := identity.GenerateLocalIdentity()
	newTransaction1 := message.New(message.EmptyId, message.EmptyId, localIdentity1, time.Now(), 0, payload.NewData([]byte("some data")))
	newTransaction2 := message.New(newTransaction1.Id(), newTransaction1.Id(), localIdentity2, time.Now(), 0, payload.NewData([]byte("some other data")))

	messageTangle.AttachMessage(newTransaction2)

	time.Sleep(7 * time.Second)

	messageTangle.AttachMessage(newTransaction1)

	messageTangle.Shutdown()
}
