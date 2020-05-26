package messagefactory

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
)

const storeSequenceInterval = 100

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events        *Events
	sequence      *kvstore.Sequence
	localIdentity *identity.LocalIdentity
	tipSelector   *tipselector.TipSelector
}

// New creates a new message factory.
func New(store kvstore.KVStore, localIdentity *identity.LocalIdentity, tipSelector *tipselector.TipSelector, sequenceKey []byte) *MessageFactory {
	sequence, err := kvstore.NewSequence(store, sequenceKey, storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &MessageFactory{
		Events:        newEvents(),
		sequence:      sequence,
		localIdentity: localIdentity,
		tipSelector:   tipSelector,
	}
}

// IssuePayload creates a new message including sequence number and tip selection and returns it.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (m *MessageFactory) IssuePayload(payload payload.Payload) *message.Message {
	sequenceNumber, err := m.sequence.Next()
	if err != nil {
		m.Events.Error.Trigger(fmt.Errorf("could not create sequence number: %w", err))
		return nil
	}

	trunkMessageId, branchMessageId := m.tipSelector.Tips()
	msg := message.New(
		trunkMessageId,
		branchMessageId,
		m.localIdentity,
		time.Now(),
		sequenceNumber,
		payload,
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
