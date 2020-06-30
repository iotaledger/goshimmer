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

// A TipSelector selects two tips, branch and trunk, for a new message to attach to.
type TipSelector interface {
	Tips() (trunk message.Id, branch message.Id)
}

// A Worker performs the PoW for the provided message in serialized byte form.
type Worker interface {
	DoPOW([]byte) (nonce uint64, err error)
}

// ZeroWorker is a PoW worker that always returns 0 as the nonce.
var ZeroWorker = WorkerFunc(func([]byte) (uint64, error) { return 0, nil })

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
func (m *MessageFactory) IssuePayload(payload payload.Payload) *message.Message {
	m.issuanceMutex.Lock()
	defer m.issuanceMutex.Unlock()
	sequenceNumber, err := m.sequence.Next()
	if err != nil {
		m.Events.Error.Trigger(fmt.Errorf("could not create sequence number: %w", err))
		return nil
	}

	trunkID, branchID := m.selector.Tips()
	issuingTime := time.Now()
	issuerPublicKey := m.localIdentity.PublicKey()

	// do the PoW
	nonce, err := m.doPOW(trunkID, branchID, issuingTime, issuerPublicKey, sequenceNumber, payload)
	if err != nil {
		m.Events.Error.Trigger(fmt.Errorf("pow failed: %w", err))
		return nil
	}

	// create the signature
	signature := m.sign(trunkID, branchID, issuingTime, issuerPublicKey, sequenceNumber, payload, nonce)

	msg := message.New(
		trunkID,
		branchID,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		payload,
		nonce,
		signature,
	)
	m.Events.MessageConstructed.Trigger(msg)
	return msg
}

// Shutdown closes the messageFactory and persists the sequence number.
func (m *MessageFactory) Shutdown() {
	if err := m.sequence.Release(); err != nil {
		m.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

func (m *MessageFactory) doPOW(trunkID message.Id, branchID message.Id, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload) (uint64, error) {
	// create a dummy message to simplify marshaling
	dummy := message.New(trunkID, branchID, issuingTime, key, seq, payload, 0, ed25519.EmptySignature).Bytes()

	m.workerMutex.RLock()
	defer m.workerMutex.RUnlock()
	return m.worker.DoPOW(dummy)
}

func (m *MessageFactory) sign(trunkID message.Id, branchID message.Id, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload, nonce uint64) ed25519.Signature {
	// create a dummy message to simplify marshaling
	dummy := message.New(trunkID, branchID, issuingTime, key, seq, payload, nonce, ed25519.EmptySignature)
	dummyBytes := dummy.Bytes()

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return m.localIdentity.Sign(dummyBytes[:contentLength])
}

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func() (message.Id, message.Id)

// Tips calls f().
func (f TipSelectorFunc) Tips() (message.Id, message.Id) {
	return f()
}

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte) (uint64, error)

// DoPOW calls f(msg).
func (f WorkerFunc) DoPOW(msg []byte) (uint64, error) {
	return f(msg)
}
