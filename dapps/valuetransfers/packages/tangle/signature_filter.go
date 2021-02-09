package tangle

import (
	"errors"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/autopeering/peer"
)

// SignatureFilter represents a filter for the Parser that filters out transactions with an invalid signature.
type SignatureFilter struct {
	onAcceptCallback      func(message *tangle.Message, peer *peer.Peer)
	onRejectCallback      func(message *tangle.Message, err error, peer *peer.Peer)
	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewSignatureFilter is the constructor of the MessageFilter.
func NewSignatureFilter() *SignatureFilter {
	return &SignatureFilter{}
}

// Filter get's called whenever a new message is received. It rejects the message, if the message is not a valid value
// message.
func (filter *SignatureFilter) Filter(message *tangle.Message, peer *peer.Peer) {
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
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (filter *SignatureFilter) OnAccept(callback func(message *tangle.Message, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	defer filter.onAcceptCallbackMutex.Unlock()

	filter.onAcceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (filter *SignatureFilter) OnReject(callback func(message *tangle.Message, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.Lock()
	defer filter.onRejectCallbackMutex.Unlock()

	filter.onRejectCallback = callback
}

// getAcceptCallback returns the callback that is executed when a message passes the filter.
func (filter *SignatureFilter) getAcceptCallback() func(message *tangle.Message, peer *peer.Peer) {
	filter.onAcceptCallbackMutex.RLock()
	defer filter.onAcceptCallbackMutex.RUnlock()

	return filter.onAcceptCallback
}

// getRejectCallback returns the callback that is executed when a message is blocked by the filter.
func (filter *SignatureFilter) getRejectCallback() func(message *tangle.Message, err error, peer *peer.Peer) {
	filter.onRejectCallbackMutex.RLock()
	defer filter.onRejectCallbackMutex.RUnlock()

	return filter.onRejectCallback
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ tangle.MessageFilter = &SignatureFilter{}
