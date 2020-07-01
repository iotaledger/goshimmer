package tangle

import (
	"errors"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tipmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/events"
)

// ValueObjectFactory acts as a factory to create new value objects.
type ValueObjectFactory struct {
	tipManager *tipmanager.TipManager
	Events     *ValueObjectFactoryEvents
}

// NewValueObjectFactory creates a new ValueObjectFactory.
func NewValueObjectFactory(tipManager *tipmanager.TipManager) *ValueObjectFactory {
	return &ValueObjectFactory{
		tipManager: tipManager,
		Events: &ValueObjectFactoryEvents{
			ValueObjectConstructed: events.NewEvent(valueObjectConstructedEvent),
		},
	}
}

// IssueTransaction creates a new value object including tip selection and returns it.
// It also triggers the ValueObjectConstructed event once it's done.
func (v *ValueObjectFactory) IssueTransaction(tx *transaction.Transaction) (valueObject *payload.Payload, err error) {
	parent1, parent2 := v.tipManager.Tips()

	// check if the tx that is supposed to be issued is a double spend
	tx.Inputs().ForEach(func(outputId transaction.OutputID) bool {
		valuetransfers.Tangle().TransactionOutput(outputId).Consume(func(output *Output) {
			if output.ConsumerCount() >= 1 {
				err = errors.New("the transaction to be issued is a double spend")
			}
		})

		return err == nil
	})
	if err != nil {
		return
	}

	valueObject = payload.New(parent1, parent2, tx)
	v.Events.ValueObjectConstructed.Trigger(valueObject)

	return
}

// ValueObjectFactoryEvents represent events happening on a ValueObjectFactory.
type ValueObjectFactoryEvents struct {
	// Fired when a value object is built including tips.
	ValueObjectConstructed *events.Event
}

func valueObjectConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.Transaction))(params[0].(*transaction.Transaction))
}
