package transactionfactory

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/goshimmer/packages/binary/identity"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"

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

func (t *TransactionFactory) BuildTransaction(payload *payload.Payload) *transaction.Transaction {
	// TODO: fill other fields: tip selection, time, sequence number, get local identity and add to singleton
	seq, _ := t.sequence.Next()
	fmt.Printf("Sequence number: %d\n", seq)

	tx := transaction.New(
		transaction.EmptyId,
		transaction.EmptyId,
		// issuer of the transaction (signs automatically)
		identity.Generate(),
		*payload,
	)

	Events.TransactionConstructed.Trigger(tx)
	return tx
}
