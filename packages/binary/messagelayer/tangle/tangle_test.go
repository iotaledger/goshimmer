package tangle

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/message/payload/data"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

func BenchmarkTangle_AttachTransaction(b *testing.B) {
	dir, err := ioutil.TempDir("", b.Name())
	require.NoError(b, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	config.Node.Set(database.CFG_DIRECTORY, dir)

	tangle := New(database.GetBadgerInstance(), []byte("TEST_BINARY_TANGLE"))
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	testIdentity := ed25119.GenerateKeyPair()

	transactionBytes := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		transactionBytes[i] = message.New(message.EmptyId, message.EmptyId, testIdentity, time.Now(), 0, data.NewData([]byte("some data")))
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

	tangle := New(database.GetBadgerInstance(), []byte("TEST_BINARY_TANGLE"))
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	tangle.Events.TransactionAttached.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *transactionmetadata.CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Message) {
			fmt.Println("ATTACHED:", transaction.GetId())
		})
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *transactionmetadata.CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Message) {
			fmt.Println("SOLID:", transaction.GetId())
		})
	}))

	tangle.Events.TransactionUnsolidifiable.Attach(events.NewClosure(func(transactionId message.Id) {
		fmt.Println("UNSOLIDIFIABLE:", transactionId)
	}))

	tangle.Events.TransactionMissing.Attach(events.NewClosure(func(transactionId message.Id) {
		fmt.Println("MISSING:", transactionId)
	}))

	tangle.Events.TransactionRemoved.Attach(events.NewClosure(func(transactionId message.Id) {
		fmt.Println("REMOVED:", transactionId)
	}))

	newTransaction1 := message.New(message.EmptyId, message.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.NewData([]byte("some data")))
	newTransaction2 := message.New(newTransaction1.GetId(), newTransaction1.GetId(), ed25119.GenerateKeyPair(), time.Now(), 0, data.NewData([]byte("some other data")))

	tangle.AttachMessage(newTransaction2)

	time.Sleep(7 * time.Second)

	tangle.AttachMessage(newTransaction1)

	tangle.Shutdown()
}
