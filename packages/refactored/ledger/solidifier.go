package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
)

type Solidifier struct {
	*Ledger
}

func NewSolidifier(ledger *Ledger) (newAvailabilityManager *Solidifier) {
	return &Solidifier{
		Ledger: ledger,
	}
}

func (s *Solidifier) checkSolidityCommand(params *params, next dataflow.Next[*params]) (err error) {
	cachedInputs := s.CachedOutputs(params.InputIDs)
	defer cachedInputs.Release()
	if params.Inputs = NewOutputs(cachedInputs.Unwrap(true)...); params.Inputs.Size() != len(cachedInputs) {
		return errors.Errorf("not all outputs of %s available: %w", params.Transaction.ID(), ErrTransactionUnsolid)
	}

	return next(params)
}

func (s *Solidifier) initializeConsumers(outputIDs OutputIDs, txID TransactionID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	_ = outputIDs.ForEach(func(outputID OutputID) (err error) {
		cachedConsumers = append(cachedConsumers, s.consumerStorage.ComputeIfAbsent(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()), func(key []byte) *Consumer {
			return NewConsumer(outputID, txID)
		}))
		return nil
	})

	return cachedConsumers
}
