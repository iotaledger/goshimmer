package builtinfilters

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// ErrInvalidSignature is returned when a message contains an invalid signature.
var ErrInvalidSignature = fmt.Errorf("invalid signature")

// MessageSignatureFilter filters messages based on whether their signatures are valid.
type MessageSignatureFilter struct {
	onAcceptCallback func(msg *message.Message, peer *peer.Peer)
	onRejectCallback func(msg *message.Message, err error, peer *peer.Peer)
	workerPool       async.WorkerPool

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewMessageSignatureFilter creates a new message signature filter.
func NewMessageSignatureFilter() *MessageSignatureFilter {
	return &MessageSignatureFilter{}
}

func (filter *MessageSignatureFilter) Filter(msg *message.Message, peer *peer.Peer) {
	filter.workerPool.Submit(func() {
		if msg.VerifySignature() {
			filter.getAcceptCallback()(msg, peer)
			return
		}
		filter.getRejectCallback()(msg, ErrInvalidSignature, peer)
	})
}

func (filter *MessageSignatureFilter) OnAccept(callback func(msg *message.Message, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	filter.onAcceptCallback = callback
	filter.onAcceptCallbackMutex.Unlock()
}

func (filter *MessageSignatureFilter) OnReject(callback func(msg *message.Message, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.Lock()
	filter.onRejectCallback = callback
	filter.onRejectCallbackMutex.Unlock()
}

func (filter *MessageSignatureFilter) Shutdown() {
	filter.workerPool.ShutdownGracefully()
}

func (filter *MessageSignatureFilter) getAcceptCallback() (result func(msg *message.Message, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.RLock()
	result = filter.onAcceptCallback
	filter.onAcceptCallbackMutex.RUnlock()
	return
}

func (filter *MessageSignatureFilter) getRejectCallback() (result func(msg *message.Message, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.RLock()
	result = filter.onRejectCallback
	filter.onRejectCallbackMutex.RUnlock()
	return
}
