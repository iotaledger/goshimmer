package meta_transaction

import (
	"fmt"
	"sync"
	"testing"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/magiconair/properties/assert"
)

func TestMetaTransaction_SettersGetters(t *testing.T) {
	shardMarker := trinary.Trytes("NPHTQORL9XKA")
	trunkTransactionHash := trinary.Trytes("99999999999999999999999999999999999999999999999999999999999999999999999999999999A")
	branchTransactionHash := trinary.Trytes("99999999999999999999999999999999999999999999999999999999999999999999999999999999B")
	head := true
	tail := true
	transactionType := trinary.Trytes("9999999999999999999999")

	transaction := New()
	transaction.SetShardMarker(shardMarker)
	transaction.SetTrunkTransactionHash(trunkTransactionHash)
	transaction.SetBranchTransactionHash(branchTransactionHash)
	transaction.SetHead(head)
	transaction.SetTail(tail)
	transaction.SetTransactionType(transactionType)

	assert.Equal(t, transaction.GetWeightMagnitude(), 0)
	assert.Equal(t, transaction.GetShardMarker(), shardMarker)
	assert.Equal(t, transaction.GetTrunkTransactionHash(), trunkTransactionHash)
	assert.Equal(t, transaction.GetBranchTransactionHash(), branchTransactionHash)
	assert.Equal(t, transaction.IsHead(), head)
	assert.Equal(t, transaction.IsTail(), tail)
	assert.Equal(t, transaction.GetTransactionType(), transactionType)
	assert.Equal(t, transaction.GetHash(), FromBytes(transaction.GetBytes()).GetHash())

	fmt.Println(transaction.GetHash())
}

func BenchmarkMetaTransaction_GetHash(b *testing.B) {
	var waitGroup sync.WaitGroup

	for i := 0; i < b.N; i++ {
		waitGroup.Add(1)

		go func() {
			New().GetHash()

			waitGroup.Done()
		}()
	}

	waitGroup.Wait()
}
