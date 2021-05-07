package value

import (
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

// unspentOutputsHandler gets the unspent outputs.
func unspentOutputsHandler(c echo.Context) error {
	var request value.UnspentOutputsRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, value.UnspentOutputsResponse{Error: err.Error()})
	}

	var unspents []value.UnspentOutput
	for _, strAddress := range request.Addresses {
		address, err := ledgerstate.AddressFromBase58EncodedString(strAddress)
		if err != nil {
			log.Info(err.Error())
			continue
		}

		outputids := make([]value.OutputID, 0)
		// get outputids by address
		cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
		cachedOutputs.Consume(func(output ledgerstate.Output) {
			cachedOutputMetadata := messagelayer.Tangle().LedgerState.OutputMetadata(output.ID())
			cachedOutputMetadata.Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() == 0 {
					// iterate balances
					var b []value.Balance
					output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
						b = append(b, value.Balance{
							Value: int64(balance),
							Color: color.String(),
						})
						return true
					})

					inclusionState := value.InclusionState{}
					txID := output.ID().TransactionID()
					txInclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(txID)
					if err != nil {
						return
					}
					messagelayer.Tangle().LedgerState.TransactionMetadata(txID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
						inclusionState.Finalized = transactionMetadata.Finalized()
					})

					inclusionState.Confirmed = txInclusionState == ledgerstate.Confirmed
					inclusionState.Rejected = txInclusionState == ledgerstate.Rejected
					inclusionState.Conflicting = len(messagelayer.Tangle().LedgerState.ConflictSet(txID)) == 0

					cachedTx := messagelayer.Tangle().LedgerState.Transaction(output.ID().TransactionID())
					var timestamp time.Time
					cachedTx.Consume(func(tx *ledgerstate.Transaction) {
						timestamp = tx.Essence().Timestamp()
					})
					outputids = append(outputids, value.OutputID{
						ID:             output.ID().Base58(),
						Balances:       b,
						InclusionState: inclusionState,
						Metadata:       value.Metadata{Timestamp: timestamp},
					})
				}
			})
		})

		unspents = append(unspents, value.UnspentOutput{
			Address:   strAddress,
			OutputIDs: outputids,
		})
	}

	return c.JSON(http.StatusOK, value.UnspentOutputsResponse{UnspentOutputs: unspents})
}
