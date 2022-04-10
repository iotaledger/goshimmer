package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region dataFlow /////////////////////////////////////////////////////////////////////////////////////////////////////

// dataFlow is a Ledger component that defines the data flow (how the different commands are chained together).
type dataFlow struct {
	// ledger contains a reference to the Ledger that created the Utils.
	ledger *Ledger
}

// newDataFlow returns a new dataFlow instance for the given Ledger.
func newDataFlow(ledger *Ledger) (new *dataFlow) {
	return &dataFlow{
		ledger,
	}
}

// storeAndProcessTransaction returns a DataFlow that stores and processes a Transaction.
func (d *dataFlow) storeAndProcessTransaction() (dataFlow *dataflow.DataFlow[*dataFlowParams]) {
	return dataflow.New[*dataFlowParams](
		d.ledger.Storage.storeTransactionCommand,
		d.processTransaction().ChainedCommand,
	)
}

// processTransaction returns a DataFlow that processes a previously stored Transaction.
func (d *dataFlow) processTransaction() (dataFlow *dataflow.DataFlow[*dataFlowParams]) {
	return dataflow.New[*dataFlowParams](
		d.ledger.booker.checkAlreadyBookedCommand,
		d.checkTransaction().ChainedCommand,
		d.ledger.booker.bookTransactionCommand,
	).WithErrorCallback(d.handleError)
}

// checkTransaction returns a DataFlow that checks the validity of a Transaction.
func (d *dataFlow) checkTransaction() (dataFlow *dataflow.DataFlow[*dataFlowParams]) {
	return dataflow.New[*dataFlowParams](
		d.ledger.validator.checkSolidityCommand,
		d.ledger.validator.checkOutputsCausallyRelatedCommand,
		d.ledger.validator.checkTransactionExecutionCommand,
	)
}

// handleError handles any kind of error that is encountered while processing the DataFlows.
func (d *dataFlow) handleError(err error, params *dataFlowParams) {
	if errors.Is(err, ErrTransactionUnsolid) {
		return
	}

	if errors.Is(err, ErrTransactionInvalid) {
		d.ledger.Events.TransactionInvalid.Trigger(&TransactionInvalidEvent{
			TransactionID: params.Transaction.ID(),
			Reason:        err,
		})

		return
	}

	d.ledger.Events.Error.Trigger(err)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region dataFlowParams ///////////////////////////////////////////////////////////////////////////////////////////////

// dataFlowParams is a container for parameters that have to be determined when booking a Transactions.
type dataFlowParams struct {
	// Transaction contains the Transaction that is being processed.
	Transaction *Transaction

	// TransactionMetadata contains the metadata of the Transaction that is being processed.
	TransactionMetadata *TransactionMetadata

	// InputIDs contains the list of OutputIDs that were referenced by the Inputs.
	InputIDs utxo.OutputIDs

	// Inputs contains the Outputs that were referenced as Inputs in the Transaction.
	Inputs Outputs

	// InputsMetadata contains the metadata of the Outputs that were referenced as Inputs in the Transaction.
	InputsMetadata OutputsMetadata

	// Consumers contains the Consumers (references from the spent Outputs) that were created by the Transaction.
	Consumers []*Consumer

	// Outputs contains the Outputs that were created by the Transaction.
	Outputs Outputs
}

// newDataFlowParams returns a new dataFlowParams instance for the given Transaction.
func newDataFlowParams(tx *Transaction) (new *dataFlowParams) {
	return &dataFlowParams{
		Transaction: tx,
		InputIDs:    utxo.NewOutputIDs(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
