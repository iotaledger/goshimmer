package meta_transaction

import (
	"sync"
	"testing"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	shardMarker           = trinary.Trytes("NPHTQORL9XKA")
	trunkTransactionHash  = trinary.Trytes("99999999999999999999999999999999999999999999999999999999999999999999999999999999A")
	branchTransactionHash = trinary.Trytes("99999999999999999999999999999999999999999999999999999999999999999999999999999999B")
	head                  = true
	tail                  = true
	transactionType       = trinary.Trytes("9999999999999999999999")
)

func newTestTransaction() *MetaTransaction {
	tx := New()
	tx.SetShardMarker(shardMarker)
	tx.SetTrunkTransactionHash(trunkTransactionHash)
	tx.SetBranchTransactionHash(branchTransactionHash)
	tx.SetHead(head)
	tx.SetTail(tail)
	tx.SetTransactionType(transactionType)

	return tx
}

func TestDoPow(t *testing.T) {
	tx := newTestTransaction()
	require.NoError(t, tx.DoProofOfWork(10))

	assert.GreaterOrEqual(t, tx.GetWeightMagnitude(), 10)
}

func TestMetaTransaction_SettersGetters(t *testing.T) {
	tx := newTestTransaction()

	assert.Equal(t, tx.GetWeightMagnitude(), 0)
	assert.Equal(t, tx.GetShardMarker(), shardMarker)
	assert.Equal(t, tx.GetTrunkTransactionHash(), trunkTransactionHash)
	assert.Equal(t, tx.GetBranchTransactionHash(), branchTransactionHash)
	assert.Equal(t, tx.IsHead(), head)
	assert.Equal(t, tx.IsTail(), tail)
	assert.Equal(t, tx.GetTransactionType(), transactionType)
	metaTx, err := FromBytes(tx.GetBytes())
	require.NoError(t, err)
	assert.Equal(t, tx.GetHash(), metaTx.GetHash())

	assert.EqualValues(t, "KKDVHBENVLQUNO9WOWWEJPBBHUSYRSRKIMZWCFCDB9RYZKYWLAYWRIBRQETBFKE9TIVWQPCKFWAMCLCAV", tx.GetHash())
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
