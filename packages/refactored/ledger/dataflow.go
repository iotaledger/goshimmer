package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
)

type DataFlow struct {
	*Ledger
}

func NewDataFlow(ledger *Ledger) *DataFlow {
	return &DataFlow{
		ledger,
	}
}

func (d *DataFlow) Setup() {
	d.TransactionBookedEvent.Attach(event.NewClosure[TransactionID](func(txID TransactionID) {
		d.CachedTransaction(txID).Consume(func(tx *Transaction) {
			_ = d.Ledger.processTransaction(tx)
		})
	}))
}

func (d *DataFlow) storeAndProcessTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.storeTransactionCommand,
		d.processTransaction().ChainedCommand,
	)
}

func (d *DataFlow) processTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.checkAlreadyBookedCommand,
		d.checkTransaction().ChainedCommand,
		d.bookTransactionCommand,
	).WithErrorCallback(func(err error, params *dataFlowParams) {
		d.ErrorEvent.Trigger(err)

		// TODO: mark Transaction as invalid and trigger invalid event
	})
}

func (d *DataFlow) checkTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.checkSolidityCommand,
		d.checkOutputsCausallyRelatedCommand,
		d.executeTransactionCommand,
	)
}

type dataFlowParams struct {
	Transaction         *Transaction
	TransactionMetadata *TransactionMetadata
	InputIDs            OutputIDs
	Inputs              Outputs
	InputsMetadata      OutputsMetadata
	Consumers           []*Consumer
	Outputs             Outputs
}
