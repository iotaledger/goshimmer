package mana

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// GetPendingMana handles the request.
func GetPendingMana(c echo.Context) error {
	var req jsonmodels.PendingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.PendingResponse{Error: err.Error()})
	}
	outputID, err := ledgerstate.OutputIDFromBase58(req.OutputID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.PendingResponse{Error: err.Error()})
	}
	pending, t := manaPlugin.PendingManaOnOutput(outputID)
	return c.JSON(http.StatusOK, jsonmodels.PendingResponse{
		Mana:      pending,
		OutputID:  outputID.Base58(),
		Timestamp: t.Unix(),
	})
}
