package transactionparser

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/identity"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
)

func BenchmarkTransactionParser_ParseBytesSame(b *testing.B) {
	txBytes := transaction.New(transaction.EmptyId, transaction.EmptyId, identity.Generate(), data.New([]byte("Test"))).GetBytes()
	txParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txParser.Parse(txBytes)
	}

	txParser.Shutdown()
}

func BenchmarkTransactionParser_ParseBytesDifferent(b *testing.B) {
	transactionBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		transactionBytes[i] = transaction.New(transaction.EmptyId, transaction.EmptyId, identity.Generate(), data.New([]byte("Test"+strconv.Itoa(i)))).GetBytes()
	}

	txParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txParser.Parse(transactionBytes[i])
	}

	txParser.Shutdown()
}

func TestTransactionParser_ParseTransaction(t *testing.T) {
	tx := transaction.New(transaction.EmptyId, transaction.EmptyId, identity.Generate(), data.New([]byte("Test")))

	txParser := New()
	txParser.Parse(tx.GetBytes())

	txParser.Events.TransactionParsed.Attach(events.NewClosure(func(tx *transaction.Transaction) {
		fmt.Println("PARSED!!!")
	}))

	txParser.Shutdown()
}
