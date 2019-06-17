package gossip

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

func BenchmarkProcessSimilarTransactionsFiltered(b *testing.B) {
<<<<<<< HEAD
	byteArray := setupTransaction(transaction.MARSHALED_TOTAL_SIZE / ternary.NUMBER_OF_TRITS_IN_A_BYTE)
=======
	byteArray := setupTransaction(meta_transaction.MARSHALLED_TOTAL_SIZE / ternary.NUMBER_OF_TRITS_IN_A_BYTE)
>>>>>>> master

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ProcessReceivedTransactionData(byteArray)
	}
}

func BenchmarkProcessSimilarTransactionsUnfiltered(b *testing.B) {
<<<<<<< HEAD
	byteArray := setupTransaction(transaction.MARSHALED_TOTAL_SIZE / ternary.NUMBER_OF_TRITS_IN_A_BYTE)
=======
	byteArray := setupTransaction(meta_transaction.MARSHALLED_TOTAL_SIZE / ternary.NUMBER_OF_TRITS_IN_A_BYTE)
>>>>>>> master

	b.ResetTimer()

	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			Events.ReceiveTransaction.Trigger(meta_transaction.FromBytes(byteArray))

			wg.Done()
		}()
	}

	wg.Wait()
}

func setupTransaction(byteArraySize int) []byte {
	byteArray := make([]byte, byteArraySize)

	for i := 0; i < len(byteArray); i++ {
		byteArray[i] = byte(i % 128)
	}

	return byteArray
}
