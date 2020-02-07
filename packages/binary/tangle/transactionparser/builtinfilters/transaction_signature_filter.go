package builtinfilters

import (
	"sync"

	"github.com/iotaledger/hive.go/async"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

type TransactionSignatureFilter struct {
	onAcceptCallback func(tx *transaction.Transaction)
	onRejectCallback func(tx *transaction.Transaction)
	workerPool       async.WorkerPool

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

func NewTransactionSignatureFilter() (result *TransactionSignatureFilter) {
	result = &TransactionSignatureFilter{}

	return
}

func (filter *TransactionSignatureFilter) Filter(tx *transaction.Transaction) {
	filter.workerPool.Submit(func() {
		if tx.VerifySignature() {
			filter.getAcceptCallback()(tx)
		} else {
			filter.getRejectCallback()(tx)
		}
	})
}

func (filter *TransactionSignatureFilter) OnAccept(callback func(tx *transaction.Transaction)) {
	filter.onAcceptCallbackMutex.Lock()
	filter.onAcceptCallback = callback
	filter.onAcceptCallbackMutex.Unlock()
}

func (filter *TransactionSignatureFilter) OnReject(callback func(tx *transaction.Transaction)) {
	filter.onRejectCallbackMutex.Lock()
	filter.onRejectCallback = callback
	filter.onRejectCallbackMutex.Unlock()
}

func (filter *TransactionSignatureFilter) Shutdown() {
	filter.workerPool.ShutdownGracefully()
}

func (filter *TransactionSignatureFilter) getAcceptCallback() (result func(tx *transaction.Transaction)) {
	filter.onAcceptCallbackMutex.RLock()
	result = filter.onAcceptCallback
	filter.onAcceptCallbackMutex.RUnlock()

	return
}

func (filter *TransactionSignatureFilter) getRejectCallback() (result func(tx *transaction.Transaction)) {
	filter.onRejectCallbackMutex.RLock()
	result = filter.onRejectCallback
	filter.onRejectCallbackMutex.RUnlock()

	return
}
