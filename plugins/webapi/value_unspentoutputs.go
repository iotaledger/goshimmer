package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/labstack/echo"
)

func init() {
	Server().POST("value/unspentOutputs", unspentOutputsHandler)
}

// unspentOutputsHandler gets the unspent outputs.
func unspentOutputsHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[ValueRoot]; exists {
		return c.JSON(http.StatusForbidden, UnspentOutputsResponse{Error: "Forbidden endpoint"})
	}

	var request UnspentOutputsRequest
	if err := c.Bind(&request); err != nil {
		Log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, UnspentOutputsResponse{Error: err.Error()})
	}

	var unspents []UnspentOutput
	for _, strAddress := range request.Addresses {
		address, err := address.FromBase58(strAddress)
		if err != nil {
			Log.Info(err.Error())
			continue
		}

		outputids := make([]OutputID, 0)
		// get outputids by address
		for id, cachedOutput := range valuetransfers.Tangle().OutputsOnAddress(address) {
			// TODO: don't do this in a for
			defer cachedOutput.Release()
			output := cachedOutput.Unwrap()
			cachedTxMeta := valuetransfers.Tangle().TransactionMetadata(output.TransactionID())
			// TODO: don't do this in a for
			defer cachedTxMeta.Release()

			if output.ConsumerCount() == 0 {
				// iterate balances
				var b []Balance
				for _, balance := range output.Balances() {
					b = append(b, Balance{
						Value: balance.Value,
						Color: balance.Color.String(),
					})
				}

				inclusionState := InclusionState{}
				if cachedTxMeta.Exists() {
					txMeta := cachedTxMeta.Unwrap()
					inclusionState.Confirmed = txMeta.Confirmed()
					inclusionState.Liked = txMeta.Liked()
					inclusionState.Rejected = txMeta.Rejected()
					inclusionState.Finalized = txMeta.Finalized()
					inclusionState.Conflicting = txMeta.Conflicting()
					inclusionState.Confirmed = txMeta.Confirmed()
				}
				outputids = append(outputids, OutputID{
					ID:             id.String(),
					Balances:       b,
					InclusionState: inclusionState,
				})
			}
		}

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
