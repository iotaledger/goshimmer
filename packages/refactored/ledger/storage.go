package ledger

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	*StorageEvents
	*Ledger

	transactionStorage         *objectstorage.ObjectStorage[utxo.Transaction]
	transactionMetadataStorage *objectstorage.ObjectStorage[*TransactionMetadata]
	outputStorage              *objectstorage.ObjectStorage[utxo.Output]

	syncutils.KRWMutex
}

func NewStorage(ledger *Ledger) (newStorage *Storage) {
	return &Storage{
		StorageEvents: newStorageEvents(),
		Ledger:        ledger,
	}
}

func (s *Storage) Setup() {
}

// Store adds a new Transaction to the ledger state. It returns a boolean that indicates whether the
// Transaction was stored, its SolidityType and an error value that contains the cause for possibly exceptions.
func (s *Storage) Store(transaction utxo.Transaction) (stored bool, err error) {
	// ComputeIfAbsent can only run atomically and acts as a Mutex for storing the Transaction
	s.CachedTransactionMetadata(transaction.ID(), func(transactionID utxo.TransactionID) *TransactionMetadata {
		s.transactionStorage.Store(transaction).Release()
		stored = true

		return NewTransactionMetadata(transactionID)
	})

	if stored {
		s.TransactionStored.Trigger(transaction)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorageEvents ////////////////////////////////////////////////////////////////////////////////////////////////

type StorageEvents struct {
	TransactionStored *events.Event
}

func newStorageEvents() *StorageEvents {
	return &StorageEvents{
		TransactionStored: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(transaction utxo.Transaction))(params[0].(utxo.Transaction))
		}),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
