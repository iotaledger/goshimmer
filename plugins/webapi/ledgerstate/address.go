package ledgerstate

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetAddressOutputsEndPoint is the handler for the /ledgerstate/addresses/:address endpoint.
func GetAddressOutputsEndPoint(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
	defer cachedOutputs.Release()

	return c.JSON(http.StatusOK, NewOutputsOnAddress(address, cachedOutputs.Unwrap()))
}

// GetAddressUnspentOutputsEndPoint is the handler for the /ledgerstate/addresses/:address/unspentOutputs endpoint.
func GetAddressUnspentOutputsEndPoint(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
	defer cachedOutputs.Release()

	return c.JSON(http.StatusOK, NewOutputsOnAddress(address, cachedOutputs.Unwrap().Filter(func(output ledgerstate.Output) (isUnspent bool) {
		messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			isUnspent = outputMetadata.ConsumerCount() == 0
		})

		return
	})))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsOnAddress /////////////////////////////////////////////////////////////////////////////////////////////

// OutputsOnAddress is the JSON model of outputs that are associated to an address.
type OutputsOnAddress struct {
	Address string   `json:"address"`
	Outputs []Output `json:"outputs"`
}

// NewOutputsOnAddress creates a JSON compatible representation of the outputs on the address.
func NewOutputsOnAddress(address ledgerstate.Address, outputs ledgerstate.Outputs) OutputsOnAddress {
	return OutputsOnAddress{
		Address: address.Base58(),
		Outputs: func() (mappedOutputs []Output) {
			mappedOutputs = make([]Output, 0)
			for _, output := range outputs {
				if output != nil {
					mappedOutputs = append(mappedOutputs, NewOutput(output))
				}
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
