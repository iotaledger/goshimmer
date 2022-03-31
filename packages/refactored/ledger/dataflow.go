package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
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
	d.TransactionBookedEvent.Attach(event.NewClosure[*TransactionBookedEvent](func(event *TransactionBookedEvent) {
		consumingTransactions := utxo.NewTransactionIDs()
		_ = event.Outputs.ForEach(func(output *Output) (err error) {
			d.CachedConsumers(output.ID()).Consume(func(consumer *Consumer) {
				consumingTransactions.Add(consumer.TransactionID())
			})
			return nil
		})

		for it := consumingTransactions.Iterator(); it.HasNext(); {
			go d.CachedTransaction(it.Next()).Consume(func(tx *Transaction) {
				_ = d.Ledger.processTransaction(tx)
			})
		}
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
		d.checkTransactionExecutionCommand,
	)
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
