package value_transaction

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/magiconair/properties/assert"
)

func TestValueTransaction_SettersGetters(t *testing.T) {
	address := ternary.Trinary("A9999999999999999999999999999999999999999999999999999999999999999999999999999999F")

	transaction := New()
	transaction.SetAddress(address)

	transactionCopy := FromMetaTransaction(transaction.MetaTransaction)
	fmt.Println(transactionCopy.GetAddress())

	assert.Equal(t, transaction.GetAddress(), address)
	//assert.Equal(t, transaction.GetHash(), FromBytes(transaction.GetBytes()).GetHash())

	fmt.Println(transaction.GetHash())
	fmt.Println(transaction.GetAddress())
}
