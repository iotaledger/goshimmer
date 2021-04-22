package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/timedqueue"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const storeSequenceInterval = 100

// region MessageFactory ///////////////////////////////////////////////////////////////////////////////////////////////

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events *MessageFactoryEvents

	tangle        *Tangle
	sequence      *kvstore.Sequence
	localIdentity *identity.LocalIdentity
	selector      TipSelector

	worker        Worker
	workerMutex   sync.RWMutex
	issuanceMutex sync.Mutex
}

// NewMessageFactory creates a new message factory.
func NewMessageFactory(tangle *Tangle, selector TipSelector) *MessageFactory {
	sequence, err := kvstore.NewSequence(tangle.Options.Store, []byte(DBSequenceNumber), storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &MessageFactory{
		Events: &MessageFactoryEvents{
			MessageConstructed: events.NewEvent(messageEventHandler),
			Error:              events.NewEvent(events.ErrorCaller),
		},

		tangle:        tangle,
		sequence:      sequence,
		localIdentity: tangle.Options.Identity,
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
func (f *MessageFactory) IssuePayload(p payload.Payload, t ...*Tangle) (*Message, error) {
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
		err = xerrors.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	strongParents, weakParents, err := f.selector.Tips(p, 2, 2)
	if err != nil {
		err = xerrors.Errorf("tips could not be selected: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	issuingTime := clock.SyncedTime()

	// due to the ParentAge check we must ensure that we set the right issuing time.
	issuingTime = f.enforceIssuingTimeForParentAge(t, strongParents, issuingTime)

	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	nonce, err := f.doPOW(strongParents, weakParents, issuingTime, issuerPublicKey, sequenceNumber, p)
	if err != nil {
		err = xerrors.Errorf("pow failed: %w", err)
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
	f.Events.MessageConstructed.Trigger(msg)
	return msg, nil
}

// IssuePayloadWithDelay creates a new message including sequence number and tip selection and returns it.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
// It is the modification of IssuePayload method that wait specified time delay after message creation and
// allows to issue requested message multiple times
func (f *MessageFactory) IssuePayloadWithDelay(p payload.Payload, delay time.Duration, repeat int, t ...*Tangle) ([]*Message, error) {
	// validate query parameters
	if delay < 0 {
		err := fmt.Errorf("time delay %d, less than zero is not allowed", delay)
		f.Events.Error.Trigger(err)
		return nil, err
	}
	if repeat <= 0 {
		err := fmt.Errorf("repeat %d, less than zero is not allowed", repeat)
		f.Events.Error.Trigger(err)
		return nil, err
	}
	payloadLen := len(p.Bytes())
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	f.issuanceMutex.Lock()
	defer f.issuanceMutex.Unlock()

	messages := make([]*Message, repeat)
	timeQueue := timedqueue.New()
	finished := make(chan bool, 1)
	// dequeue and issue msg after time delay
	go func() {
		time.Sleep(delay)
		var msgCount int
		for timeQueue.Size() > 0 {
			msg := timeQueue.Poll(false).(*Message)
			if msg == nil {
				continue
			}
			messages[msgCount] = msg
			f.Events.MessageConstructed.Trigger(messages[msgCount])
			msgCount++
		}
		finished <- true
	}()
	// issue message repeat times
	for i := 0; i < repeat; i++ {
		sequenceNumber, err := f.sequence.Next()
		if err != nil {
			err = xerrors.Errorf("could not create sequence number: %w", err)
			f.Events.Error.Trigger(err)
			return nil, err
		}

		strongParents, weakParents, err := f.selector.Tips(p, 2, 2)
		if err != nil {
			err = xerrors.Errorf("tips could not be selected: %w", err)
			f.Events.Error.Trigger(err)
			return nil, err
		}

		issuingTime := clock.SyncedTime()

		// due to the ParentAge check we must ensure that we set the right issuing time.
		issuingTime = f.enforceIssuingTimeForParentAge(t, strongParents, issuingTime)

		issuerPublicKey := f.localIdentity.PublicKey()

		// do the PoW
		nonce, err := f.doPOW(strongParents, weakParents, issuingTime, issuerPublicKey, sequenceNumber, p)
		if err != nil {
			err = xerrors.Errorf("pow failed: %w", err)
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
		timeQueue.Add(msg, msg.issuingTime.Add(delay))
	}
	timeout := 2 * delay
	select {
	case <-finished:
		return messages, nil
	case <-time.After(timeout):
		return nil, xerrors.Errorf("not all messages issued after one additional delay")
	}
}

// enforceIssuingTimeForParentAge make sure the issuing time is correct with the parent age check
func (f *MessageFactory) enforceIssuingTimeForParentAge(t []*Tangle, strongParents MessageIDs, issuingTime time.Time) time.Time {
	if t != nil {
		for _, parent := range strongParents {
			t[0].Storage.Message(parent).Consume(func(msg *Message) {
				if msg.ID() != EmptyMessageID && !msg.IssuingTime().Before(issuingTime) {
					time.Sleep(msg.IssuingTime().Sub(issuingTime) + 1*time.Nanosecond)
					issuingTime = clock.SyncedTime()
				}
			})
		}
	}
	return issuingTime
}

// Shutdown closes the MessageFactory and persists the sequence number.
func (f *MessageFactory) Shutdown() {
	if err := f.sequence.Release(); err != nil {
		f.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

func (f *MessageFactory) doPOW(strongParents []MessageID, weakParents []MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload) (uint64, error) {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(strongParents, weakParents, issuingTime, key, seq, payload, 0, ed25519.EmptySignature).Bytes()

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy)
}

func (f *MessageFactory) sign(strongParents []MessageID, weakParents []MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, payload payload.Payload, nonce uint64) ed25519.Signature {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(strongParents, weakParents, issuingTime, key, seq, payload, nonce, ed25519.EmptySignature)
	dummyBytes := dummy.Bytes()

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.localIdentity.Sign(dummyBytes[:contentLength])
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageFactoryEvents /////////////////////////////////////////////////////////////////////////////////////////

// MessageFactoryEvents represents events happening on a message factory.
type MessageFactoryEvents struct {
	// Fired when a message is built including tips, sequence number and other metadata.
	MessageConstructed *events.Event

	// Fired when an error occurred.
	Error *events.Event
}

func messageEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*Message))(params[0].(*Message))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelector //////////////////////////////////////////////////////////////////////////////////////////////////

// A TipSelector selects two tips, parent2 and parent1, for a new message to attach to.
type TipSelector interface {
	Tips(p payload.Payload, countStrongParents, countWeakParents int) (strongParents, weakParents MessageIDs, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelectorFunc //////////////////////////////////////////////////////////////////////////////////////////////

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func(p payload.Payload, countStrongParents, countWeakParents int) (strongParents, weakParents MessageIDs, err error)

// Tips calls f().
func (f TipSelectorFunc) Tips(p payload.Payload, countStrongParents, countWeakParents int) (strongParents, weakParents MessageIDs, err error) {
	return f(p, countStrongParents, countWeakParents)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Worker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// A Worker performs the PoW for the provided message in serialized byte form.
type Worker interface {
	DoPOW([]byte) (nonce uint64, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WorkerFunc ///////////////////////////////////////////////////////////////////////////////////////////////////

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte) (uint64, error)

// DoPOW calls f(msg).
func (f WorkerFunc) DoPOW(msg []byte) (uint64, error) {
	return f(msg)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ZeroWorker ///////////////////////////////////////////////////////////////////////////////////////////////////

// ZeroWorker is a PoW worker that always returns 0 as the nonce.
var ZeroWorker = WorkerFunc(func([]byte) (uint64, error) { return 0, nil })

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
