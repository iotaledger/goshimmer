package transactionfactory

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
	instance *TransactionFactory
	log      *logger.Logger
)

type TransactionFactory struct {
	sequence *badger.Sequence
}

func Setup(logger *logger.Logger, db *badger.DB, sequenceKey []byte) *TransactionFactory {
	once.Do(func() {
		log = logger
		sequence, err := db.GetSequence(sequenceKey, 100)
		if err != nil {
			log.Fatalf("Could not create transaction sequence number. %v", err)
		}

		instance = &TransactionFactory{
			sequence: sequence,
		}
	})

	return instance
}

func GetInstance() *TransactionFactory {
	return instance
}

func (t *TransactionFactory) Shutdown() {
	if err := t.sequence.Release(); err != nil {
		log.Errorf("Could not release transaction sequence number. %v", err)
	}
}

func (t *TransactionFactory) BuildTransaction(payload *payload.Payload) *message.Transaction {
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

	Events.TransactionConstructed.Trigger(tx)
	return tx
}
