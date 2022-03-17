package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	TransactionStoredEvent     *event.Event[*TransactionStoredEvent]
	transactionStorage         *objectstorage.ObjectStorage[utxo.Transaction]
	transactionMetadataStorage *objectstorage.ObjectStorage[*TransactionMetadata]
	consumerStorage            *objectstorage.ObjectStorage[*Consumer]
	outputStorage              *objectstorage.ObjectStorage[utxo.Output]

	*Ledger
}

func NewStorage(ledger *Ledger) (newStorage *Storage) {
	return &Storage{
		TransactionStoredEvent: event.New[*TransactionStoredEvent](),
		Ledger:                 ledger,
	}
}

func (s *Storage) Setup() {
}

// Store adds a new Transaction to the ledger state. It returns a boolean that indicates whether the
// Transaction was stored.
func (s *Storage) Store(transaction utxo.Transaction) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata], stored bool) {
	cachedTransactionMetadata = s.CachedTransactionMetadata(transaction.ID(), func(transactionID utxo.TransactionID) *TransactionMetadata {
		s.transactionStorage.Store(transaction).Release()
		stored = true
		return NewTransactionMetadata(transactionID)
	})

	if !stored {
		return
	}

	return
}

// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
func (s *Storage) CachedTransaction(transactionID utxo.TransactionID) (cachedTransaction *objectstorage.CachedObject[utxo.Transaction]) {
	return s.transactionStorage.Load(transactionID.Bytes())
}

// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (s *Storage) CachedTransactionMetadata(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *TransactionMetadata) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionMetadataStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) *TransactionMetadata {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionMetadataStorage.Load(transactionID.Bytes())
}

// CachedOutput retrieves the Output with the given OutputID from the object storage.
func (s *Storage) CachedOutput(outputID utxo.OutputID) (cachedOutput *objectstorage.CachedObject[utxo.Output]) {
	return s.outputStorage.Load(outputID.Bytes())
}

// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
func (u *Storage) CachedConsumers(outputID utxo.OutputID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	u.consumerStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Consumer]) bool {
		cachedConsumers = append(cachedConsumers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(outputID.Bytes()))

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
