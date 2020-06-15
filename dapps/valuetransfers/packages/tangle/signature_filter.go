package tangle

import (
	"errors"
	"sync"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messageparser"
)

// SignatureFilter represents a filter for the MessageParser that filters out transactions with an invalid signature.
type SignatureFilter struct {
	onAcceptCallback func(message *message.Message, peer *peer.Peer)
	onRejectCallback func(message *message.Message, err error, peer *peer.Peer)
	workerPool       async.WorkerPool

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewSignatureFilter is the constructor of the MessageFilter.
func NewSignatureFilter() (result *SignatureFilter) {
	result = &SignatureFilter{}

	return
}

// Filter get's called whenever a new message is received. It first checks if the message contains a value Payload and
// then verifies the signature.
func (filter *SignatureFilter) Filter(message *message.Message, peer *peer.Peer) {
	filter.workerPool.Submit(func() {
		// accept message if the message is not a value message (it will be checked by other filters)
		valuePayload := message.Payload()
		if valuePayload.Type() != payload.Type {
			filter.getAcceptCallback()(message, peer)

			return
		}

		// reject if the payload can not be casted to a ValuePayload (invalid payload)
		typeCastedValuePayload, ok := valuePayload.(*payload.Payload)
		if !ok {
			filter.getRejectCallback()(message, errors.New("invalid value message"), peer)

			return
		}

		// reject message if it contains a transaction with invalid signatures
		if !typeCastedValuePayload.Transaction().SignaturesValid() {
			filter.getRejectCallback()(message, errors.New("invalid transaction signatures"), peer)

			return
		}

		// if all previous checks passed: accept message
		filter.getAcceptCallback()(message, peer)
	})
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (filter *SignatureFilter) OnAccept(callback func(message *message.Message, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	filter.onAcceptCallback = callback
	filter.onAcceptCallbackMutex.Unlock()
}

// OnAccept registers the given callback as the rejection function of the filter.
func (filter *SignatureFilter) OnReject(callback func(message *message.Message, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.Lock()
	filter.onRejectCallback = callback
	filter.onRejectCallbackMutex.Unlock()
}

// Shutdown shuts down the filter.
func (filter *SignatureFilter) Shutdown() {
	filter.workerPool.ShutdownGracefully()
}

// getAcceptCallback returns the callback that is be executed when a message passes the filter.
func (filter *SignatureFilter) getAcceptCallback() (result func(message *message.Message, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.RLock()
	result = filter.onAcceptCallback
	filter.onAcceptCallbackMutex.RUnlock()

	return
}

// getRejectCallback returns the callback that is executed when a message is blocked by the filter.
func (filter *SignatureFilter) getRejectCallback() (result func(message *message.Message, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.RLock()
	result = filter.onRejectCallback
	filter.onRejectCallbackMutex.RUnlock()

	return
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ messageparser.MessageFilter = &SignatureFilter{}
