package realitiesledger

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/core/dataflow"
)

// region dataFlow /////////////////////////////////////////////////////////////////////////////////////////////////////

// dataFlow is a RealitiesLedger component that defines the data flow (how the different commands are chained together).
type dataFlow struct {
	// ledger contains a reference to the RealitiesLedger that created the Utils.
	ledger *RealitiesLedger
}

// newDataFlow returns a new dataFlow instance for the given RealitiesLedger.
func newDataFlow(ledger *RealitiesLedger) *dataFlow {
	return &dataFlow{
		ledger,
	}
}

// storeAndProcessTransaction returns a DataFlow that stores and processes a Transaction.
func (d *dataFlow) storeAndProcessTransaction() (dataFlow *dataflow.DataFlow[*dataFlowParams]) {
	return dataflow.New(
		d.ledger.storage.storeTransactionCommand,
		d.processTransaction().ChainedCommand,
	)
}

// processTransaction returns a DataFlow that processes a previously stored Transaction.
func (d *dataFlow) processTransaction() (dataFlow *dataflow.DataFlow[*dataFlowParams]) {
	return dataflow.New(
		d.ledger.booker.checkAlreadyBookedCommand,
		d.checkTransaction().ChainedCommand,
		d.ledger.booker.bookTransactionCommand,
	).WithErrorCallback(d.handleError)
}

// checkTransaction returns a DataFlow that checks the validity of a Transaction.
func (d *dataFlow) checkTransaction() (dataFlow *dataflow.DataFlow[*dataFlowParams]) {
	return dataflow.New(
		d.ledger.validator.checkSolidityCommand,
		d.ledger.validator.checkOutputsCausallyRelatedCommand,
		d.ledger.validator.checkTransactionExecutionCommand,
	)
}

// handleError handles any kind of error that is encountered while processing the DataFlows.
func (d *dataFlow) handleError(err error, params *dataFlowParams) {
	if errors.Is(err, mempool.ErrTransactionUnsolid) {
		return
	}

	if errors.Is(err, mempool.ErrTransactionInvalid) {
		d.ledger.events.TransactionInvalid.Trigger(&mempool.TransactionInvalidEvent{
			TransactionID: params.Transaction.ID(),
			Reason:        err,
			Context:       params.Context,
		})

		return
	}

	d.ledger.events.Error.Trigger(err)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region dataFlowParams ///////////////////////////////////////////////////////////////////////////////////////////////

// dataFlowParams is a container for parameters that have to be determined when booking a Transactions.
type dataFlowParams struct {
	// Context can be handed in by external callers and gets passed through in the events.
	Context context.Context

	// Transaction contains the Transaction that is being processed.
	Transaction utxo.Transaction

	// TransactionMetadata contains the metadata of the Transaction that is being processed.
	TransactionMetadata *mempool.TransactionMetadata

	// InputIDs contains the list of OutputIDs that were referenced by the Inputs.
	InputIDs utxo.OutputIDs

	// Inputs contains the Outputs that were referenced as Inputs in the Transaction.
	Inputs *utxo.Outputs

	// InputsMetadata contains the metadata of the Outputs that were referenced as Inputs in the Transaction.
	InputsMetadata *mempool.OutputsMetadata

	// Consumers contains the Consumers (references from the spent Outputs) that were created by the Transaction.
	Consumers []*mempool.Consumer

	// Outputs contains the Outputs that were created by the Transaction.
	Outputs *utxo.Outputs
}

// newDataFlowParams returns a new dataFlowParams instance for the given Transaction.
func newDataFlowParams(ctx context.Context, tx utxo.Transaction) *dataFlowParams {
	return &dataFlowParams{
		Context:     ctx,
		Transaction: tx,
		InputIDs:    utxo.NewOutputIDs(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
