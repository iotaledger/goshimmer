package ledgerstate

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetAddressOutputsEndPoint is the handler for the /ledgerstate/addresses/:address endpoint.
func GetAddressOutputsEndPoint(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
	defer cachedOutputs.Release()

	outputs := cachedOutputs.Unwrap()
	for _, output := range outputs {
		if output == nil {
			return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("failed to load outputs")))
		}
	}

	return c.JSON(http.StatusOK, NewOutputsOnAddress(outputs))
}

// GetAddressUnspentOutputsEndPoint is the handler for the /ledgerstate/addresses/:address/unspentOutputs endpoint.
func GetAddressUnspentOutputsEndPoint(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
	defer cachedOutputs.Release()

	outputs := cachedOutputs.Unwrap()
	unspentOutputs := make(ledgerstate.Outputs, 0)
	for _, output := range outputs {
		if output == nil {
			return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("failed to load outputs")))
		}
		messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			if outputMetadata.ConsumerCount() == 0 {
				unspentOutputs = append(unspentOutputs, output)
			}
		})
	}

	return c.JSON(http.StatusOK, NewUnspentOutputsOnAddress(unspentOutputs))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsOnAddress /////////////////////////////////////////////////////////////////////////////////////////////

// OutputsOnAddress is the JSON model of outputs that are associated to an address.
type OutputsOnAddress struct {
	OutputCount int      `json:"outputsCount"`
	Outputs     []Output `json:"outputs"`
}

// NewOutputsOnAddress creates a JSON compatible representation of the outputs on the address.
func NewOutputsOnAddress(outputs ledgerstate.Outputs) OutputsOnAddress {
	return OutputsOnAddress{
		OutputCount: len(outputs),
		Outputs: func() []Output {
			jsonOutputs := make([]Output, len(outputs))
			for i, output := range outputs {
				jsonOutputs[i] = NewOutput(output)
			}
			return jsonOutputs
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnspentOutputsOnAddress //////////////////////////////////////////////////////////////////////////////////////

// UnspentOutputsOnAddress is the JSON model of unspent outputs that are associated to an address.
type UnspentOutputsOnAddress struct {
	UnspentOutputsCount int      `json:"unspentOutputsCount"`
	UnspentOutputs      []Output `json:"unspentOutputs"`
}

// NewUnspentOutputsOnAddress creates a JSON compatible representation of the unspent outputs on the address.
func NewUnspentOutputsOnAddress(unspentOutputs ledgerstate.Outputs) UnspentOutputsOnAddress {
	return UnspentOutputsOnAddress{
		UnspentOutputsCount: len(unspentOutputs),
		UnspentOutputs: func() []Output {
			jsonOutputs := make([]Output, len(unspentOutputs))
			for i, output := range unspentOutputs {
				jsonOutputs[i] = NewOutput(output)
			}
			return jsonOutputs
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
