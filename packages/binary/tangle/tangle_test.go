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

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
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

	testIdentity := identity.GenerateLocalIdentity()

	transactionBytes := make([]*transaction.Transaction, b.N)
	for i := 0; i < b.N; i++ {
		transactionBytes[i] = transaction.New(transaction.EmptyId, transaction.EmptyId, testIdentity.PublicKey(), time.Now(), 0, data.New([]byte("some data")), testIdentity)
		transactionBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.AttachTransaction(transactionBytes[i])
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

	tangle.Events.TransactionAttached.Attach(events.NewClosure(func(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *transactionmetadata.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			fmt.Println("ATTACHED:", transaction.GetId())
		})
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *transactionmetadata.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			fmt.Println("SOLID:", transaction.GetId())
		})
	}))

	tangle.Events.TransactionUnsolidifiable.Attach(events.NewClosure(func(transactionId transaction.Id) {
		fmt.Println("UNSOLIDIFIABLE:", transactionId)
	}))

	tangle.Events.TransactionMissing.Attach(events.NewClosure(func(transactionId transaction.Id) {
		fmt.Println("MISSING:", transactionId)
	}))

	tangle.Events.TransactionRemoved.Attach(events.NewClosure(func(transactionId transaction.Id) {
		fmt.Println("REMOVED:", transactionId)
	}))

	localIdentity1 := identity.GenerateLocalIdentity()
	localIdentity2 := identity.GenerateLocalIdentity()
	newTransaction1 := transaction.New(transaction.EmptyId, transaction.EmptyId, localIdentity1.PublicKey(), time.Now(), 0, data.New([]byte("some data")), localIdentity1)
	newTransaction2 := transaction.New(newTransaction1.GetId(), newTransaction1.GetId(), localIdentity2.PublicKey(), time.Now(), 0, data.New([]byte("some other data")), localIdentity2)

	tangle.AttachTransaction(newTransaction2)

	time.Sleep(7 * time.Second)

	tangle.AttachTransaction(newTransaction1)

	tangle.Shutdown()
}
