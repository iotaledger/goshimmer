package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region dataFlow /////////////////////////////////////////////////////////////////////////////////////////////////////

type dataFlow struct {
	*Ledger
}

func newDataFlow(ledger *Ledger) *dataFlow {
	return &dataFlow{
		ledger,
	}
}

func (d *dataFlow) storeAndProcessTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.Storage.storeTransactionCommand,
		d.processTransaction().ChainedCommand,
	)
}

func (d *dataFlow) processTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.booker.checkAlreadyBookedCommand,
		d.checkTransaction().ChainedCommand,
		d.booker.bookTransactionCommand,
	).WithErrorCallback(func(err error, params *dataFlowParams) {
		d.Events.Error.Trigger(err)

		// TODO: mark Transaction as invalid and trigger invalid event
	})
}

func (d *dataFlow) checkTransaction() *dataflow.DataFlow[*dataFlowParams] {
	return dataflow.New[*dataFlowParams](
		d.validator.checkSolidityCommand,
		d.validator.checkOutputsCausallyRelatedCommand,
		d.validator.checkTransactionExecutionCommand,
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region dataFlowParams ///////////////////////////////////////////////////////////////////////////////////////////////

func newDataFlowParams(tx *Transaction) (new *dataFlowParams) {
	return &dataFlowParams{
		Transaction: tx,
	}
}

type dataFlowParams struct {
	Transaction         *Transaction
	TransactionMetadata *TransactionMetadata
	InputIDs            utxo.OutputIDs
	Inputs              Outputs
	InputsMetadata      OutputsMetadata
	Consumers           []*Consumer
	Outputs             Outputs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
