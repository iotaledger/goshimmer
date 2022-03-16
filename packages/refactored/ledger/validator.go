package ledger

import (
	"container/list"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Validator struct{}

func (v *Validator) IsValid() {

}

func (v *Validator) isPastConeValid(inputs []utxo.Output, inputsMetadata []*OutputMetadata) bool {
	if u.outputsUnspent(outputsMetadata) {
		pastConeValid = true
		return
	}

	stack := list.New()
	consumedInputIDs := make(map[OutputID]types.Empty)
	for _, input := range outputs {
		consumedInputIDs[input.ID()] = types.Void
		stack.PushBack(input.ID())
	}

	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		cachedConsumers := u.CachedConsumers(firstElement.Value.(OutputID))
		for _, consumer := range cachedConsumers.Unwrap() {
			if consumer == nil {
				cachedConsumers.Release()
				panic("failed to unwrap Consumer")
			}

			cachedTransaction := u.CachedTransaction(consumer.TransactionID())
			transaction, exists := cachedTransaction.Unwrap()
			if !exists {
				cachedTransaction.Release()
				cachedConsumers.Release()
				panic("failed to unwrap Transaction")
			}

			for _, output := range transaction.Essence().Outputs() {
				if _, exists := consumedInputIDs[output.ID()]; exists {
					cachedTransaction.Release()
					cachedConsumers.Release()
					return false
				}

				stack.PushBack(output.ID())
			}

			cachedTransaction.Release()
		}
		cachedConsumers.Release()
	}

	return true
}
