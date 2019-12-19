package ui

import (
	"strings"
	"sync"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/iota.go/transaction"
)

// cleared every second
var transactions []transaction.Transaction
var tMutex sync.Mutex

var emptyTag = strings.Repeat("9", 27)

func logTransactions() interface{} {
	tMutex.Lock()
	defer tMutex.Unlock()
	a := transactions
	transactions = make([]transaction.Transaction, 0)
	return a
}

func saveTx(tx *value_transaction.ValueTransaction) {
	tMutex.Lock()
	defer tMutex.Unlock()
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
