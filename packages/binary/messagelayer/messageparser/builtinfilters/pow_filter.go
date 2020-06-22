package builtinfilters

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"
)

var (
	// ErrInvalidPOWDifficultly is returned when the nonce of a message does not fulfill the PoW difficulty.
	ErrInvalidPOWDifficultly = errors.New("invalid PoW")
	// ErrMessageTooSmall is returned when the message does not contain enough data for the PoW.
	ErrMessageTooSmall = errors.New("message too small")
)

// PowFilter is a message bytes filter validating the PoW nonce.
type PowFilter struct {
	worker     *pow.Worker
	difficulty int
	workerPool async.WorkerPool

	mu             sync.Mutex
	acceptCallback func([]byte, *peer.Peer)
	rejectCallback func([]byte, error, *peer.Peer)
}

// NewPowFilter creates a new PoW bytes filter.
func NewPowFilter(worker *pow.Worker, difficulty int) *PowFilter {
	return &PowFilter{
		worker:     worker,
		difficulty: difficulty,
	}
}

// Filter checks whether the given bytes pass the PoW validation and calls the corresponding callback.
func (f *PowFilter) Filter(msgBytes []byte, p *peer.Peer) {
	f.workerPool.Submit(func() {
		if err := f.validate(msgBytes); err != nil {
			f.reject(msgBytes, err, p)
			return
		}
		f.accept(msgBytes, p)
	})
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *PowFilter) OnAccept(callback func([]byte, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *PowFilter) OnReject(callback func([]byte, error, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectCallback = callback
}

// Shutdown shuts down the filter.
func (f *PowFilter) Shutdown() {
	f.workerPool.ShutdownGracefully()
}

func (f *PowFilter) accept(msgBytes []byte, p *peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.acceptCallback != nil {
		f.acceptCallback(msgBytes, p)
	}
}

func (f *PowFilter) reject(msgBytes []byte, err error, p *peer.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rejectCallback != nil {
		f.rejectCallback(msgBytes, err, p)
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

// powData returns the bytes over which PoW should be computed.
func powData(msgBytes []byte) ([]byte, error) {
	contentLength := len(msgBytes) - ed25519.SignatureSize
	if contentLength < pow.NonceBytes {
		return nil, ErrMessageTooSmall
	}
	return msgBytes[:contentLength], nil
}
