package mana

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getPendingHandler handles the request.
func getPendingHandler(c echo.Context) error {
	var req PendingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, PendingResponse{Error: err.Error()})
	}
	outputID, err := ledgerstate.OutputIDFromBase58(req.OutputID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, PendingResponse{Error: err.Error()})
	}
	pending, t := manaPlugin.PendingManaOnOutput(outputID)
	return c.JSON(http.StatusOK, PendingResponse{
		Mana:      pending,
		OutputID:  outputID.Base58(),
		Timestamp: t.Unix(),
	})
}

// PendingRequest is the pending mana request.
type PendingRequest struct {
	OutputID string `json:"outputID"`
}

// PendingResponse is the pending mana response.
type PendingResponse struct {
	Mana      float64 `json:"mana"`
	OutputID  string  `json:"outputID"`
	Error     string  `json:"error,omitempty"`
	Timestamp int64   `json:"timestamp"`
}
