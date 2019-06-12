package value_transaction

import (
	"fmt"
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

	transaction := New()
	transaction.SetShardMarker(shardMarker)
	transaction.SetTrunkTransactionHash(trunkTransactionHash)
	transaction.SetBranchTransactionHash(branchTransactionHash)
	transaction.SetHead(head)
	transaction.SetTail(tail)

	assert.Equal(t, transaction.GetWeightMagnitude(), 0)
	assert.Equal(t, transaction.GetShardMarker(), shardMarker)
	assert.Equal(t, transaction.GetTrunkTransactionHash(), trunkTransactionHash)
	assert.Equal(t, transaction.GetBranchTransactionHash(), branchTransactionHash)
	assert.Equal(t, transaction.GetHead(), head)
	assert.Equal(t, transaction.GetTail(), tail)
	//assert.Equal(t, transaction.GetHash(), FromBytes(transaction.GetBytes()).GetHash())

	fmt.Println(transaction.GetHash())
}
