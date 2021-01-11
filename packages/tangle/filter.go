package tangle

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/bytesfilter"
	"github.com/iotaledger/hive.go/identity"
)

var (
	// ErrInvalidPOWDifficultly is returned when the nonce of a message does not fulfill the PoW difficulty.
	ErrInvalidPOWDifficultly = errors.New("invalid PoW")

	// ErrMessageTooSmall is returned when the message does not contain enough data for the PoW.
	ErrMessageTooSmall = errors.New("message too small")

	// ErrInvalidSignature is returned when a message contains an invalid signature.
	ErrInvalidSignature = fmt.Errorf("invalid signature")

	// ErrReceivedDuplicateBytes is returned when duplicated bytes are rejected.
	ErrReceivedDuplicateBytes = fmt.Errorf("received duplicate bytes")
)

// BytesFilter filters based on byte slices and peers.
type BytesFilter interface {
	// Filter filters up on the given bytes and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(bytes []byte, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(bytes []byte, peer *peer.Peer))
	// OnReject registers the given callback as the rejection function of the filter.
	OnReject(callback func(bytes []byte, err error, peer *peer.Peer))
}

// MessageFilter filters based on messages and peers.
type MessageFilter interface {
	// Filter filters up on the given message and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(msg *Message, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(msg *Message, peer *peer.Peer))
	// OnAccept registers the given callback as the rejection function of the filter.
	OnReject(callback func(msg *Message, err error, peer *peer.Peer))
}

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

// PowFilter is a message bytes filter validating the PoW nonce.
type PowFilter struct {
	worker     *pow.Worker
	difficulty int

	nodesMessages      map[identity.ID]*pow.MessagesWindow
	nodesMessagesMutex sync.Mutex

	mu             sync.Mutex
	acceptCallback func(*Message, *peer.Peer)
	rejectCallback func(*Message, error, *peer.Peer)
}

// NewPowFilter creates a new PoW bytes filter.
func NewPowFilter(worker *pow.Worker, difficulty int) *PowFilter {
	pow.BaseDifficulty = difficulty
	return &PowFilter{
		worker:        worker,
		difficulty:    difficulty,
		nodesMessages: make(map[identity.ID]*pow.MessagesWindow),
	}
}

// Filter checks whether the given bytes pass the PoW validation and calls the corresponding callback.
func (f *PowFilter) Filter(msg *Message, p *peer.Peer) {
	zeros, err := f.leadingZeros(msg)
	if err != nil {
		f.reject(msg, err, p)
		return
	}

	f.nodesMessagesMutex.Lock()
	if _, exist := f.nodesMessages[p.ID()]; !exist {
		f.nodesMessages[p.ID()] = &pow.MessagesWindow{}
	}
	messagesWindow := f.nodesMessages[p.ID()]
	f.nodesMessagesMutex.Unlock()

	if !messagesWindow.CheckDifficulty(
		pow.MessageAge{
			ID:        msg.ID().String(),
			Timestamp: msg.IssuingTime(),
		}, zeros) {
		f.reject(msg, ErrInvalidPOWDifficultly, p)
		return
	}

	f.accept(msg, p)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *PowFilter) OnAccept(callback func(*Message, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *PowFilter) OnReject(callback func(*Message, error, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectCallback = callback
}

func (f *PowFilter) accept(msg *Message, p *peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.acceptCallback != nil {
		f.acceptCallback(msg, p)
	}
}

func (f *PowFilter) reject(msg *Message, err error, p *peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rejectCallback != nil {
		f.rejectCallback(msg, err, p)
	}
}

func (f *PowFilter) validate(msgBytes []byte) error {
	content, err := powData(msgBytes)
	if err != nil {
		return err
	}
	zeros, err := f.worker.LeadingZeros(content)
	if err != nil {
		return err
	}
	if zeros < f.difficulty {
		return fmt.Errorf("%w: leading zeros %d for difficulty %d", ErrInvalidPOWDifficultly, zeros, f.difficulty)
	}
	return nil
}

func (f *PowFilter) leadingZeros(msg *Message) (int, error) {
	zeros := 0
	if msg == nil {
		return zeros, ErrMessageTooSmall
	}
	content, err := powData(msg.Bytes())
	if err != nil {
		return zeros, err
	}
	return f.worker.LeadingZeros(content)
}

// powData returns the bytes over which PoW should be computed.
func powData(msgBytes []byte) ([]byte, error) {
	contentLength := len(msgBytes) - ed25519.SignatureSize
	if contentLength < pow.NonceBytes {
		return nil, ErrMessageTooSmall
	}
	return msgBytes[:contentLength], nil
}

// RecentlySeenBytesFilter filters so that bytes which were recently seen don't pass the filter.
type RecentlySeenBytesFilter struct {
	bytesFilter      *bytesfilter.BytesFilter
	onAcceptCallback func(bytes []byte, peer *peer.Peer)
	onRejectCallback func(bytes []byte, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewRecentlySeenBytesFilter creates a new recently seen bytes filter.
func NewRecentlySeenBytesFilter() *RecentlySeenBytesFilter {
	return &RecentlySeenBytesFilter{
		bytesFilter: bytesfilter.New(100000),
	}
}

// Filter filters up on the given bytes and peer and calls the acceptance callback
// if the input passes or the rejection callback if the input is rejected.
func (f *RecentlySeenBytesFilter) Filter(bytes []byte, peer *peer.Peer) {
	if f.bytesFilter.Add(bytes) {
		f.getAcceptCallback()(bytes, peer)
		return
	}
	f.getRejectCallback()(bytes, ErrReceivedDuplicateBytes, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *RecentlySeenBytesFilter) OnAccept(callback func(bytes []byte, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	f.onAcceptCallback = callback
	f.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *RecentlySeenBytesFilter) OnReject(callback func(bytes []byte, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	f.onRejectCallback = callback
	f.onRejectCallbackMutex.Unlock()
}

func (f *RecentlySeenBytesFilter) getAcceptCallback() (result func(bytes []byte, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	result = f.onAcceptCallback
	f.onAcceptCallbackMutex.Unlock()
	return
}

func (f *RecentlySeenBytesFilter) getRejectCallback() (result func(bytes []byte, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	result = f.onRejectCallback
	f.onRejectCallbackMutex.Unlock()
	return
}
