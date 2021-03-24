package mana

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

func getOnlineAccessHandler(c echo.Context) error {
	return getOnlineHandler(c, mana.AccessMana)
}

func getOnlineConsensusHandler(c echo.Context) error {
	return getOnlineHandler(c, mana.ConsensusMana)
}

// getOnlineHandler handles the request.
func getOnlineHandler(c echo.Context, manaType mana.Type) error {
	onlinePeersMana, t, err := manaPlugin.GetOnlineNodes(manaType)
	if err != nil {
		return c.JSON(http.StatusNotFound, GetOnlineResponse{Error: err.Error()})
	}
	resp := make([]OnlineNodeStr, 0)
	for index, value := range onlinePeersMana {
		resp = append(resp, OnlineNodeStr{
			OnlineRank: index + 1,
			ShortID:    value.ID.String(),
			ID:         base58.Encode(value.ID.Bytes()),
			Mana:       value.Mana,
		})
	}

	return c.JSON(http.StatusOK, GetOnlineResponse{
		Online:    resp,
		Timestamp: t.Unix(),
	})
}

// GetOnlineResponse is the response to an online mana request.
type GetOnlineResponse struct {
	Online    []OnlineNodeStr `json:"online"`
	Error     string          `json:"error,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// OnlineNodeStr holds information about online rank, nodeID and mana,
type OnlineNodeStr struct {
	OnlineRank int     `json:"rank"`
	ShortID    string  `json:"shortNodeID"`
	ID         string  `json:"nodeID"`
	Mana       float64 `json:"mana"`
}
