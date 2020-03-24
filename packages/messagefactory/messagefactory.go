package messagefactory

import (
	"sync"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/tipselector"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/hive.go/logger"
)

var (
	once     sync.Once
	instance *MessageFactory
	log      *logger.Logger
)

type MessageFactory struct {
	sequence      *badger.Sequence
	localIdentity *identity.LocalIdentity
	tipSelector   *tipselector.TipSelector
}

func Setup(logger *logger.Logger, db *badger.DB, localIdentity *identity.LocalIdentity, tipSelector *tipselector.TipSelector, sequenceKey []byte) *MessageFactory {
	once.Do(func() {
		log = logger
		sequence, err := db.GetSequence(sequenceKey, 100)
		if err != nil {
			log.Fatalf("Could not create transaction sequence number. %v", err)
		}

		instance = &MessageFactory{
			sequence:      sequence,
			localIdentity: localIdentity,
			tipSelector:   tipSelector,
		}
	})

	return instance
}

func GetInstance() *MessageFactory {
	return instance
}

// Shutdown closes the  messageFactory and persists the sequence number
func (m *MessageFactory) Shutdown() {
	if err := m.sequence.Release(); err != nil {
		log.Errorf("Could not release transaction sequence number. %v", err)
	}
}

// BuildMessage constructs a new message with sequence number and performs tip selection and returns it.
// It triggers MessageConstructed event once it's done.
func (m *MessageFactory) BuildMessage(payload payload.Payload) *message.Transaction {
	sequenceNumber, err := m.sequence.Next()
	if err != nil {
		panic("Could not create sequence number")
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

	Events.MessageConstructed.Trigger(tx)
	return tx
}
