package tangle

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
)

// ErrInvalidSignature is returned when a message contains an invalid signature.
var ErrInvalidSignature = fmt.Errorf("invalid signature")

// MessageSignatureFilter filters messages based on whether their signatures are valid.
type MessageSignatureFilter struct {
	onAcceptCallback func(msg *Message, peer *peer.Peer)
	onRejectCallback func(msg *Message, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewMessageSignatureFilter creates a new message signature filter.
func NewMessageSignatureFilter() *MessageSignatureFilter {
	return &MessageSignatureFilter{}
}

// Filter filters up on the given bytes and peer and calls the acceptance callback
// if the input passes or the rejection callback if the input is rejected.
func (f *MessageSignatureFilter) Filter(msg *Message, peer *peer.Peer) {
	if msg.VerifySignature() {
		f.getAcceptCallback()(msg, peer)
		return
	}
	f.getRejectCallback()(msg, ErrInvalidSignature, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *MessageSignatureFilter) OnAccept(callback func(msg *Message, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	f.onAcceptCallback = callback
	f.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *MessageSignatureFilter) OnReject(callback func(msg *Message, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	f.onRejectCallback = callback
	f.onRejectCallbackMutex.Unlock()
}

func (f *MessageSignatureFilter) getAcceptCallback() (result func(msg *Message, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.RLock()
	result = f.onAcceptCallback
	f.onAcceptCallbackMutex.RUnlock()
	return
}

func (f *MessageSignatureFilter) getRejectCallback() (result func(msg *Message, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.RLock()
	result = f.onRejectCallback
	f.onRejectCallbackMutex.RUnlock()
	return
}
