package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
)

type DataFlow struct {
	*Ledger
}

func NewDataFlow(ledger *Ledger) *DataFlow {
	return &DataFlow{
		ledger,
	}
}

func (d *DataFlow) storeAndProcessTransaction() *dataflow.DataFlow[*params] {
	return dataflow.New[*params](
		d.storeTransactionCommand,
		d.processTransaction().ChainedCommand,
	)
}

func (d *DataFlow) processTransaction() *dataflow.DataFlow[*params] {
	return dataflow.New[*params](
		d.initConsumersCommand,
		d.checkTransaction().ChainedCommand,
		d.bookTransactionCommand,
	).WithErrorCallback(func(err error, params *params) {
		d.ErrorEvent.Trigger(err)

		// TODO: mark Transaction as invalid and trigger invalid event
	})
}

func (d *DataFlow) checkTransaction() *dataflow.DataFlow[*params] {
	return dataflow.New[*params](
		d.checkSolidityCommand,
		d.checkOutputsCausallyRelatedCommand,
		d.executeTransactionCommand,
	)
}
