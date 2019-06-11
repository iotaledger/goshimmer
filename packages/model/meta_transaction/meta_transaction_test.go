package meta_transaction

import (
	"fmt"
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/magiconair/properties/assert"
)

func TestMetaTransaction_SettersGetters(t *testing.T) {
	shardMarker := ternary.Trinary("NPHTQORL9XKA")
	trunkTransactionHash := ternary.Trinary("99999999999999999999999999999999999999999999999999999999999999999999999999999999A")
	branchTransactionHash := ternary.Trinary("99999999999999999999999999999999999999999999999999999999999999999999999999999999B")
	head := true
	tail := true
	transactionType := ternary.Trinary("9999999999999999999999")

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
	assert.Equal(t, transaction.GetHead(), head)
	assert.Equal(t, transaction.GetTail(), tail)
	assert.Equal(t, transaction.GetTransactionType(), transactionType)
	assert.Equal(t, transaction.GetHash(), FromBytes(transaction.GetBytes()).GetHash())

	fmt.Println(transaction.GetHash())
}

func BenchmarkMetaTransaction_GetHash(b *testing.B) {
	var waitGroup sync.WaitGroup

	for i := 0; i < b.N; i++ {
		go func() {
			waitGroup.Add(1)

			New().GetHash()

			waitGroup.Done()
		}()
	}

	waitGroup.Wait()
}
