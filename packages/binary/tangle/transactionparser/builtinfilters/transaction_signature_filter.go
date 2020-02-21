package builtinfilters

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

var ErrInvalidSignature = fmt.Errorf("invalid signature")

type TransactionSignatureFilter struct {
	onAcceptCallback func(tx *transaction.Transaction, peer *peer.Peer)
	onRejectCallback func(tx *transaction.Transaction, err error, peer *peer.Peer)
	workerPool       async.WorkerPool

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

func NewTransactionSignatureFilter() (result *TransactionSignatureFilter) {
	result = &TransactionSignatureFilter{}

	return
}

func (filter *TransactionSignatureFilter) Filter(tx *transaction.Transaction, peer *peer.Peer) {
	filter.workerPool.Submit(func() {
		if tx.VerifySignature() {
			filter.getAcceptCallback()(tx, peer)
		} else {
			filter.getRejectCallback()(tx, ErrInvalidSignature, peer)
		}
	})
}

func (filter *TransactionSignatureFilter) OnAccept(callback func(tx *transaction.Transaction, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	filter.onAcceptCallback = callback
	filter.onAcceptCallbackMutex.Unlock()
}

func (filter *TransactionSignatureFilter) OnReject(callback func(tx *transaction.Transaction, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.Lock()
	filter.onRejectCallback = callback
	filter.onRejectCallbackMutex.Unlock()
}

func (filter *TransactionSignatureFilter) Shutdown() {
	filter.workerPool.ShutdownGracefully()
}

func (filter *TransactionSignatureFilter) getAcceptCallback() (result func(tx *transaction.Transaction, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.RLock()
	result = filter.onAcceptCallback
	filter.onAcceptCallbackMutex.RUnlock()

	return
}

func (filter *TransactionSignatureFilter) getRejectCallback() (result func(tx *transaction.Transaction, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.RLock()
	result = filter.onRejectCallback
	filter.onRejectCallbackMutex.RUnlock()

	return
}
