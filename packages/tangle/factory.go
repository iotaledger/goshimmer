package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
)

const storeSequenceInterval = 100

var (
	// ZeroWorker is a PoW worker that always returns 0 as the nonce.
	ZeroWorker = WorkerFunc(func([]byte) (uint64, error) { return 0, nil })
)

// A TipSelector selects two tips, parent2 and parent1, for a new message to attach to.
type TipSelector interface {
	Tips() (parent1 MessageID, parent2 MessageID)
}

// A Worker performs the PoW for the provided message in serialized byte form.
type Worker interface {
	DoPOW([]byte) (nonce uint64, err error)
}

// Factory acts as a factory to create new messages.
type Factory struct {
	Events        *FactoryEvents
	sequence      *kvstore.Sequence
	localIdentity *identity.LocalIdentity
	selector      TipSelector

	worker        Worker
	workerMutex   sync.RWMutex
	issuanceMutex sync.Mutex
}

// NewFactory creates a new message factory.
func NewFactory(store kvstore.KVStore, sequenceKey []byte, localIdentity *identity.LocalIdentity, selector TipSelector) *Factory {
	sequence, err := kvstore.NewSequence(store, sequenceKey, storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &Factory{
		Events:        newFactoryEvents(),
		sequence:      sequence,
		localIdentity: localIdentity,
		selector:      selector,
		worker:        ZeroWorker,
	}
}

// SetWorker sets the PoW worker to be used for the messages.
func (f *Factory) SetWorker(worker Worker) {
	f.workerMutex.Lock()
	defer f.workerMutex.Unlock()
	f.worker = worker
}

// IssuePayload creates a new message including sequence number and tip selection and returns it.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (f *Factory) IssuePayload(p Payload) (*Message, error) {
	payloadLen := len(p.Bytes())
	if payloadLen > MaxPayloadSize {
		err := fmt.Errorf("%w: %d bytes", ErrMaxPayloadSizeExceeded, payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	f.issuanceMutex.Lock()
	defer f.issuanceMutex.Unlock()
	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		err = fmt.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	parent1ID, parent2ID := f.selector.Tips()
	issuingTime := time.Now()
	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	nonce, err := f.doPOW(parent1ID, parent2ID, issuingTime, issuerPublicKey, sequenceNumber, p)
	if err != nil {
		err = fmt.Errorf("pow failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature := f.sign(parent1ID, parent2ID, issuingTime, issuerPublicKey, sequenceNumber, p, nonce)

	msg := NewMessage(
		parent1ID,
		parent2ID,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)
	f.Events.MessageConstructed.Trigger(msg)
	return msg, nil
}

// Shutdown closes the messageFactory and persists the sequence number.
func (f *Factory) Shutdown() {
	if err := f.sequence.Release(); err != nil {
		f.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

func (f *Factory) doPOW(parent1ID MessageID, parent2ID MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload Payload) (uint64, error) {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(parent1ID, parent2ID, issuingTime, key, seq, payload, 0, ed25519.EmptySignature).Bytes()

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy)
}

func (f *Factory) sign(parent1ID MessageID, parent2ID MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload Payload, nonce uint64) ed25519.Signature {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(parent1ID, parent2ID, issuingTime, key, seq, payload, nonce, ed25519.EmptySignature)
	dummyBytes := dummy.Bytes()

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.localIdentity.Sign(dummyBytes[:contentLength])
}

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func() (MessageID, MessageID)

// Tips calls f().
func (f TipSelectorFunc) Tips() (MessageID, MessageID) {
	return f()
}

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte) (uint64, error)

// DoPOW calls f(msg).
func (f WorkerFunc) DoPOW(msg []byte) (uint64, error) {
	return f(msg)
}
