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

func (d *DataFlow) storeAndProcessTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.storeTransactionCommand,
		d.processTransaction().ChainedCommand,
	)
}

func (d *DataFlow) processTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.initConsumersCommand,
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
