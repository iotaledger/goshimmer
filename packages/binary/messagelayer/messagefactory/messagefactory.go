package messagefactory

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
)

type MessageFactory struct {
	Events        *Events
	sequence      *badger.Sequence
	localIdentity *identity.LocalIdentity
	tipSelector   *tipselector.TipSelector
}

func New(db *badger.DB, localIdentity *identity.LocalIdentity, tipSelector *tipselector.TipSelector, sequenceKey []byte) *MessageFactory {
	sequence, err := db.GetSequence(sequenceKey, 100)
	if err != nil {
		panic(fmt.Errorf("Could not create transaction sequence number. %v", err))
	}

	return &MessageFactory{
		Events:        newEvents(),
		sequence:      sequence,
		localIdentity: localIdentity,
		tipSelector:   tipSelector,
	}
}

// BuildMessage constructs a new message with sequence number and performs tip selection and returns it.
// It triggers MessageConstructed event once it's done.
func (m *MessageFactory) BuildMessage(payload payload.Payload) *message.Message {
	sequenceNumber, err := m.sequence.Next()
	if err != nil {
		m.Events.Error.Trigger(errors.Wrap(err, "Could not create sequence number"))

		return nil
	}

	trunkTransaction, branchTransaction := m.tipSelector.GetTips()

	tx := message.New(
		trunkTransaction,
		branchTransaction,
		m.localIdentity.PublicKey(),
		time.Now(),
		sequenceNumber,
		payload,
		m.localIdentity,
	)

	m.Events.MessageConstructed.Trigger(tx)

	return tx
}

// Shutdown closes the  messageFactory and persists the sequence number
func (m *MessageFactory) Shutdown() {
	if err := m.sequence.Release(); err != nil {
		m.Events.Error.Trigger(errors.Wrap(err, "Could not release transaction sequence number."))
	}
}
