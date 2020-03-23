package messsagefactory

import (
	"sync"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/hive.go/logger"
)

var (
	once     sync.Once
	instance *MessageFactory
	log      *logger.Logger
)

type MessageFactory struct {
	sequence *badger.Sequence
}

func Setup(logger *logger.Logger, db *badger.DB, sequenceKey []byte) *MessageFactory {
	once.Do(func() {
		log = logger
		sequence, err := db.GetSequence(sequenceKey, 100)
		if err != nil {
			log.Fatalf("Could not create transaction sequence number. %v", err)
		}

		instance = &MessageFactory{
			sequence: sequence,
		}
	})

	return instance
}

func GetInstance() *MessageFactory {
	return instance
}

func (t *MessageFactory) Shutdown() {
	if err := t.sequence.Release(); err != nil {
		log.Errorf("Could not release transaction sequence number. %v", err)
	}
}

func (t *MessageFactory) BuildMessage(payload *payload.Payload) *message.Transaction {
	// TODO: fill other fields: tip selection, time, sequence number, get local identity and add to singleton
	seq, err := t.sequence.Next()
	if err != nil {
		panic("Could not create sequence number")
	}

	localIdentity := identity.GenerateLocalIdentity()
	tx := message.New(
		// trunk in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// branch in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// issuer of the transaction (signs automatically)
		localIdentity.PublicKey(),

		// the time when the transaction was created
		time.Now(),

		// the ever increasing sequence number of this transaction
		seq,

		// payload
		*payload,

		localIdentity,
	)

	Events.MessageConstructed.Trigger(tx)
	return tx
}
