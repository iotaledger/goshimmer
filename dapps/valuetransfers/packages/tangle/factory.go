package tangle

import (
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
func (v *ValueObjectFactory) IssueTransaction(tx *transaction.Transaction) *payload.Payload {
	parent1, parent2 := v.tipManager.Tips()

	valueObject := payload.New(parent1, parent2, tx)
	v.Events.ValueObjectConstructed.Trigger(valueObject)

	return valueObject
}

// ValueObjectFactoryEvents represent events happening on a ValueObjectFactory.
type ValueObjectFactoryEvents struct {
	// Fired when a value object is built including tips.
	ValueObjectConstructed *events.Event
}

func valueObjectConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.Transaction))(params[0].(*transaction.Transaction))
}
