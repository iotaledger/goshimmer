package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
)

const storeSequenceInterval = 100

var (
	// ZeroWorker is a PoW worker that always returns 0 as the nonce.
	ZeroWorker = WorkerFunc(func([]byte, int) (uint64, error) { return 0, nil })
)

// A TipSelector selects two tips, parent2 and parent1, for a new message to attach to.
type TipSelector interface {
	Tips(count int) (parents []MessageID)
}

// A Worker performs the PoW for the provided message in serialized byte form.
type Worker interface {
	DoPOW([]byte, int) (nonce uint64, err error)
}

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events        *MessageFactoryEvents
	sequence      *kvstore.Sequence
	localIdentity *identity.LocalIdentity
	selector      TipSelector
	mw            pow.MessagesWindow

	worker        Worker
	workerMutex   sync.RWMutex
	issuanceMutex sync.Mutex
}

// NewMessageFactory creates a new message factory.
func NewMessageFactory(store kvstore.KVStore, sequenceKey []byte, localIdentity *identity.LocalIdentity, selector TipSelector) *MessageFactory {
	sequence, err := kvstore.NewSequence(store, sequenceKey, storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &MessageFactory{
		Events:        newMessageFactoryEvents(),
		sequence:      sequence,
		localIdentity: localIdentity,
		selector:      selector,
		worker:        ZeroWorker,
	}
}

// SetWorker sets the PoW worker to be used for the messages.
func (f *MessageFactory) SetWorker(worker Worker) {
	f.workerMutex.Lock()
	defer f.workerMutex.Unlock()
	f.worker = worker
}

// IssuePayload creates a new message including sequence number and tip selection and returns it.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (f *MessageFactory) IssuePayload(p payload.Payload) (*Message, error) {
	payloadLen := len(p.Bytes())
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
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

	// TODO: change hardcoded amount of parents
	strongParents := f.selector.Tips(2)
	// TODO: approval switch: select weak parents
	weakParents := make([]MessageID, 0)

	issuingTime := clock.SyncedTime()
	issuerPublicKey := f.localIdentity.PublicKey()

	// Calculate the current difficulty for this msg.
	currentDifficulty := f.mw.AdaptiveDifficulty(issuingTime)

	// do the PoW
	nonce, err := f.doPOW(strongParents, weakParents, issuingTime, issuerPublicKey, sequenceNumber, p, currentDifficulty)
	if err != nil {
		err = fmt.Errorf("pow failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature := f.sign(strongParents, weakParents, issuingTime, issuerPublicKey, sequenceNumber, p, nonce)

	msg := NewMessage(
		strongParents,
		weakParents,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)

	// Update our messagesWindow
	f.mw.Append(pow.MessageAge{
		ID:        msg.ID().String(),
		Timestamp: issuingTime,
	})

	f.Events.MessageConstructed.Trigger(msg)
	return msg, nil
}

// Shutdown closes the MessageFactory and persists the sequence number.
func (f *MessageFactory) Shutdown() {
	if err := f.sequence.Release(); err != nil {
		f.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

func (f *MessageFactory) doPOW(strongParents []MessageID, weakParents []MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload, difficulty int) (uint64, error) {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(strongParents, weakParents, issuingTime, key, seq, payload, 0, ed25519.EmptySignature).Bytes()

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy, difficulty)
}

func (f *MessageFactory) sign(strongParents []MessageID, weakParents []MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload, nonce uint64) ed25519.Signature {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(strongParents, weakParents, issuingTime, key, seq, payload, nonce, ed25519.EmptySignature)
	dummyBytes := dummy.Bytes()

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.localIdentity.Sign(dummyBytes[:contentLength])
}

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func(count int) (parents []MessageID)

// Tips calls f().
func (f TipSelectorFunc) Tips(count int) (parents []MessageID) {
	return f(count)
}

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte, int) (uint64, error)

// DoPOW calls f(msg).
func (f WorkerFunc) DoPOW(msg []byte, difficulty int) (uint64, error) {
	return f(msg, difficulty)
}
