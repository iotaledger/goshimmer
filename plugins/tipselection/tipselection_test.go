package tipselection

import (
	"log"
	"testing"

	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/logger"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	if err := logger.InitGlobalLogger(viper.New()); err != nil {
		log.Fatal(err)
	}
}

func TestEmptyTipSet(t *testing.T) {
	configure(nil)
	assert.Equal(t, 0, GetTipsCount())
	assert.Equal(t, meta_transaction.BRANCH_NULL_HASH, GetRandomTip())
}

func TestSingleTip(t *testing.T) {
	configure(nil)

	tx := value_transaction.New()
	tx.SetValue(int64(1))
	tx.SetBranchTransactionHash(meta_transaction.BRANCH_NULL_HASH)
	tx.SetTrunkTransactionHash(meta_transaction.BRANCH_NULL_HASH)

	tangle.Events.TransactionSolid.Trigger(tx)

	assert.Equal(t, 1, GetTipsCount())

	tip1 := GetRandomTip()
	assert.NotNil(t, tip1)
	tip2 := GetRandomTip(tip1)
	assert.NotNil(t, tip2)
	assert.Equal(t, tip1, tip2)
}

func TestGetRandomTip(t *testing.T) {
	configure(nil)

	tx := value_transaction.New()
	tx.SetValue(int64(1))
	tx.SetBranchTransactionHash(meta_transaction.BRANCH_NULL_HASH)
	tx.SetTrunkTransactionHash(meta_transaction.BRANCH_NULL_HASH)

	tangle.Events.TransactionSolid.Trigger(tx)

	tx = value_transaction.New()
	tx.SetValue(int64(2))
	tx.SetBranchTransactionHash(meta_transaction.BRANCH_NULL_HASH)
	tx.SetTrunkTransactionHash(meta_transaction.BRANCH_NULL_HASH)

	tangle.Events.TransactionSolid.Trigger(tx)

	assert.Equal(t, 2, GetTipsCount())

	tip1 := GetRandomTip()
	require.NotNil(t, tip1)
	tip2 := GetRandomTip(tip1)
	require.NotNil(t, tip2)
	require.NotEqual(t, tip1, tip2)

	tx = value_transaction.New()
	tx.SetValue(int64(3))
	tx.SetBranchTransactionHash(tip1)
	tx.SetTrunkTransactionHash(tip2)

	tangle.Events.TransactionSolid.Trigger(tx)

	assert.Equal(t, 1, GetTipsCount())
	assert.Equal(t, tx.GetHash(), GetRandomTip())
}
