package mana

import (
	"net/http"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
)

// getPastConsensusManaVectorHandler handles the request.
func getPastConsensusManaVectorHandler(c echo.Context) error {
	var req PastConsensusManaVectorRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, PendingResponse{Error: err.Error()})
	}
	timestamp := time.Unix(req.Timestamp, 0)
	consensus, _, err := manaPlugin.GetPastConsensusManaVector(timestamp.Add(1 * time.Second))
	if err != nil {
		return c.JSON(http.StatusBadRequest, PastConsensusManaVectorResponse{Error: err.Error()})
	}
	manaMap, _, err := consensus.GetManaMap(timestamp)
	if err != nil {
		return c.JSON(http.StatusBadRequest, PastConsensusManaVectorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, PastConsensusManaVectorResponse{
		Consensus: manaMap.ToNodeStrList(),
		TimeStamp: timestamp.Unix(),
	})
}

// PastConsensusManaVectorRequest is the request.
type PastConsensusManaVectorRequest struct {
	Timestamp int64 `json:"timestamp"`
}

// PastConsensusManaVectorResponse is the response.
type PastConsensusManaVectorResponse struct {
	Consensus []mana.NodeStr `json:"consensus"`
	Error     string         `json:"error,omitempty"`
	TimeStamp int64          `json:"timestamp"`
}
