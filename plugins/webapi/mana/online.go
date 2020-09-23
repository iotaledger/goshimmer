package mana

import (
	"net/http"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

func getOnlineAccessHandler(c echo.Context) error {
	return getOnlineHandler(c, manaPkg.AccessMana)
}

func getOnlineConsensusHandler(c echo.Context) error {
	return getOnlineHandler(c, manaPkg.ConsensusMana)
}

// getOnlineHandler handles the request.
func getOnlineHandler(c echo.Context, manaType manaPkg.Type) error {
	onlinePeersMana, err := manaPlugin.GetOnlineNodes(manaType)
	if err != nil {
		return c.JSON(http.StatusNotFound, GetOnlineResponse{Error: err.Error()})
	}
	resp := make([]OnlineNodeStr, 0)
	for index, value := range onlinePeersMana {
		resp = append(resp, OnlineNodeStr{OnlineRank: index + 1, NodeID: value.ID.String(), Mana: value.Mana})
	}

	return c.JSON(http.StatusOK, GetOnlineResponse{Online: resp})
}

// GetOnlineResponse is the response to an online mana request.
type GetOnlineResponse struct {
	Online []OnlineNodeStr `json:"online"`
	Error  string          `json:"error,omitempty"`
}

// OnlineNodeStr holds information about online rank, nodeID and mana,
type OnlineNodeStr struct {
	OnlineRank int     `json:"rank"`
	NodeID     string  `json:"nodeID"`
	Mana       float64 `json:"mana"`
}
