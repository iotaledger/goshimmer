package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tipmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/events"
)

type ValueObjectFactory struct {
	branchManager *branchmanager.BranchManager
	tipManager    *tipmanager.TipManager
	Events        *ValueObjectFactoryEvents
}

func NewValueObjectFactory(branchManager *branchmanager.BranchManager, tipManager *tipmanager.TipManager) *ValueObjectFactory {
	return &ValueObjectFactory{
		branchManager: branchManager,
		tipManager:    tipManager,
		Events: &ValueObjectFactoryEvents{
			ValueObjectConstructed: events.NewEvent(valueObjectConstructedEvent),
		},
	}
}

func (v *ValueObjectFactory) IssueTransaction(tx *transaction.Transaction) *payload.Payload {
	// TODO: we need a way to get the currently liked branches. e.g. branchManager.LikedBranches()
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
