package messagefactory

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
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
	Tips() (parent1 message.ID, parent2 message.ID)
}

// A Worker performs the PoW for the provided message in serialized byte form.
type Worker interface {
	DoPOW([]byte) (nonce uint64, err error)
}

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events        *Events
	sequence      *kvstore.Sequence
	localIdentity *identity.LocalIdentity
	selector      TipSelector

	worker        Worker
	workerMutex   sync.RWMutex
	issuanceMutex sync.Mutex
}

// New creates a new message factory.
func New(store kvstore.KVStore, sequenceKey []byte, localIdentity *identity.LocalIdentity, selector TipSelector) *MessageFactory {
	sequence, err := kvstore.NewSequence(store, sequenceKey, storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &MessageFactory{
		Events:        newEvents(),
		sequence:      sequence,
		localIdentity: localIdentity,
		selector:      selector,
		worker:        ZeroWorker,
	}
}

// SetWorker sets the PoW worker to be used for the messages.
func (m *MessageFactory) SetWorker(worker Worker) {
	m.workerMutex.Lock()
	defer m.workerMutex.Unlock()
	m.worker = worker
}

// IssuePayload creates a new message including sequence number and tip selection and returns it.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (m *MessageFactory) IssuePayload(p payload.Payload) (*message.Message, error) {
	payloadLen := len(p.Bytes())
	if payloadLen > payload.MaxPayloadSize {
		err := fmt.Errorf("%w: %d bytes", payload.ErrMaxPayloadSizeExceeded, payloadLen)
		m.Events.Error.Trigger(err)
		return nil, err
	}

	m.issuanceMutex.Lock()
	defer m.issuanceMutex.Unlock()
	sequenceNumber, err := m.sequence.Next()
	if err != nil {
		err = fmt.Errorf("could not create sequence number: %w", err)
		m.Events.Error.Trigger(err)
		return nil, err
	}

	parent1ID, parent2ID := m.selector.Tips()
	issuingTime := time.Now()
	issuerPublicKey := m.localIdentity.PublicKey()

	// do the PoW
	nonce, err := m.doPOW(parent1ID, parent2ID, issuingTime, issuerPublicKey, sequenceNumber, p)
	if err != nil {
		err = fmt.Errorf("pow failed: %w", err)
		m.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature := m.sign(parent1ID, parent2ID, issuingTime, issuerPublicKey, sequenceNumber, p, nonce)

	msg := message.New(
		parent1ID,
		parent2ID,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)
	m.Events.MessageConstructed.Trigger(msg)
	return msg, nil
}

// Shutdown closes the messageFactory and persists the sequence number.
func (m *MessageFactory) Shutdown() {
	if err := m.sequence.Release(); err != nil {
		m.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

func (m *MessageFactory) doPOW(parent1ID message.ID, parent2ID message.ID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload) (uint64, error) {
	// create a dummy message to simplify marshaling
	dummy := message.New(parent1ID, parent2ID, issuingTime, key, seq, payload, 0, ed25519.EmptySignature).Bytes()

	m.workerMutex.RLock()
	defer m.workerMutex.RUnlock()
	return m.worker.DoPOW(dummy)
}

func (m *MessageFactory) sign(parent1ID message.ID, parent2ID message.ID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload, nonce uint64) ed25519.Signature {
	// create a dummy message to simplify marshaling
	dummy := message.New(parent1ID, parent2ID, issuingTime, key, seq, payload, nonce, ed25519.EmptySignature)
	dummyBytes := dummy.Bytes()

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return m.localIdentity.Sign(dummyBytes[:contentLength])
}

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func() (message.ID, message.ID)

// Tips calls f().
func (f TipSelectorFunc) Tips() (message.ID, message.ID) {
	return f()
}

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte) (uint64, error)

// DoPOW calls f(msg).
func (f WorkerFunc) DoPOW(msg []byte) (uint64, error) {
	return f(msg)
}
