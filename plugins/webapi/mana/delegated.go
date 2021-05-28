package mana

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/plugins/manarefresher"
)

// region GetDelegatedMana /////////////////////////////////////////////////////////////////////////////////////////////

// GetDelegatedMana handles the GetDelegatedMana request.
func GetDelegatedMana(c echo.Context) error {
	delegatedMana := manarefresher.TotalDelegatedFunds()
	return c.JSON(http.StatusOK, &GetDelegatedManaResponse{DelegatedMana: delegatedMana})
}

// GetDelegatedManaResponse is the response struct for the GetDelegatedMana endpoint.
type GetDelegatedManaResponse struct {
	DelegatedMana uint64 `json:"delegatedMana"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetDelegatedOutputs /////////////////////////////////////////////////////////////////////////////////////////////////

// GetDelegatedOutputs handles the GetDelegatedOutputs requests.
func GetDelegatedOutputs(c echo.Context) error {
	outputs, err := manarefresher.DelegatedOutputs()
	if err != nil {
		return c.JSON(http.StatusNotFound, &GetDelegatedOutputsResponse{Error: err.Error()})
	}
	delegatedOutputsJSON := make([]*jsonmodels.Output, len(outputs))
	for i, o := range outputs {
		delegatedOutputsJSON[i] = jsonmodels.NewOutput(o)
	}
	return c.JSON(http.StatusOK, &GetDelegatedOutputsResponse{Outputs: delegatedOutputsJSON})
}

// GetDelegatedOutputsResponse is the response struct for the GetDelegatedOutputs endpoint.
type GetDelegatedOutputsResponse struct {
	Outputs []*jsonmodels.Output `json:"delegatedOutputs"`
	Error   string               `json:"error,omitempty"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
