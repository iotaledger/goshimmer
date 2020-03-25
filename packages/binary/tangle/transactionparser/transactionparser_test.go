package transactionparser

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload/data"
)

func BenchmarkTransactionParser_ParseBytesSame(b *testing.B) {
	txBytes := message.New(message.EmptyId, message.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("Test"))).Bytes()
	txParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txParser.Parse(txBytes, nil)
	}

	txParser.Shutdown()
}

func BenchmarkTransactionParser_ParseBytesDifferent(b *testing.B) {
	transactionBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		transactionBytes[i] = message.New(message.EmptyId, message.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("Test"+strconv.Itoa(i)))).Bytes()
	}

	txParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txParser.Parse(transactionBytes[i], nil)
	}

	txParser.Shutdown()
}

func TestTransactionParser_ParseTransaction(t *testing.T) {
	tx := message.New(message.EmptyId, message.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("Test")))

	txParser := New()
	txParser.Parse(tx.Bytes(), nil)

	txParser.Events.TransactionParsed.Attach(events.NewClosure(func(tx *message.Transaction) {
		fmt.Println("PARSED!!!")
	}))

	txParser.Shutdown()
}
