package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// unspentOutputsHandler gets the unspent outputs.
func unspentOutputsHandler(c echo.Context) error {
	var request UnspentOutputsRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, UnspentOutputsResponse{Error: err.Error()})
	}

	var unspents []UnspentOutput
	for _, strAddress := range request.Addresses {
		address, err := ledgerstate.AddressFromBase58EncodedString(strAddress)
		if err != nil {
			log.Info(err.Error())
			continue
		}

		outputids := make([]OutputID, 0)
		// get outputids by address
		cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
		cachedOutputs.Consume(func(output ledgerstate.Output) {
			cachedOutputMetadata := messagelayer.Tangle().LedgerState.OutputMetadata(output.ID())
			cachedOutputMetadata.Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() == 0 {
					// iterate balances
					var b []Balance
					output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
						b = append(b, Balance{
							Value: int64(balance),
							Color: color.String(),
						})
						return true
					})

					inclusionState := InclusionState{}
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

					outputids = append(outputids, OutputID{
						ID:             output.ID().Base58(),
						Balances:       b,
						InclusionState: inclusionState,
					})
				}
			})
		})

		unspents = append(unspents, UnspentOutput{
			Address:   strAddress,
			OutputIDs: outputids,
		})
	}

	return c.JSON(http.StatusOK, UnspentOutputsResponse{UnspentOutputs: unspents})
}

// UnspentOutputsRequest holds the addresses to query.
type UnspentOutputsRequest struct {
	Addresses []string `json:"addresses,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// UnspentOutputsResponse is the HTTP response from retrieving value objects.
type UnspentOutputsResponse struct {
	UnspentOutputs []UnspentOutput `json:"unspent_outputs,omitempty"`
	Error          string          `json:"error,omitempty"`
}
