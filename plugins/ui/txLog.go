package ui

import (
	"strings"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/iota.go/transaction"
)

// cleared every second
var transactions []transaction.Transaction

var emptyTag = strings.Repeat("9", 27)

func logTransactions() interface{} {
	a := transactions
	transactions = make([]transaction.Transaction, 0)
	return a
}

func saveTx(tx *value_transaction.ValueTransaction) {
	transactions = append(transactions, transaction.Transaction{
		Hash:              tx.MetaTransaction.GetHash(),
		Address:           tx.GetAddress(),
		Value:             tx.GetValue(),
		Timestamp:         uint64(tx.GetTimestamp()),
		TrunkTransaction:  tx.MetaTransaction.GetTrunkTransactionHash(),
		BranchTransaction: tx.MetaTransaction.GetBranchTransactionHash(),
		Tag:               emptyTag,
	})
}
