package mana

import (
	"net/http"
	"strconv"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getNHighestAccessHandler handles a /mana/access/nhighest request.
func getNHighestAccessHandler(c echo.Context) error {
	return nHighestHandler(c, manaPkg.AccessMana)
}

// getNHighestConsensusHandler handles a /mana/consensus/nhighest request.
func getNHighestConsensusHandler(c echo.Context) error {
	return nHighestHandler(c, manaPkg.ConsensusMana)
}

// nHighestHandler handles the request.
func nHighestHandler(c echo.Context, manaType manaPkg.Type) error {
	number, err := strconv.ParseUint(c.QueryParam("number"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetNHighestResponse{Error: err.Error()})
	}
	highestNodes, err := manaPlugin.GetHighestManaNodes(manaType, uint(number), manaPkg.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetNHighestResponse{Error: err.Error()})
	}
	var res []manaPkg.NodeStr
	for _, n := range highestNodes {
		res = append(res, n.ToNodeStr())
	}
	return c.JSON(http.StatusOK, GetNHighestResponse{
		Nodes: res,
	})
}

// GetNHighestResponse holds info about nodes and their mana values.
type GetNHighestResponse struct {
	Error string            `json:"error,omitempty"`
	Nodes []manaPkg.NodeStr `json:"nodes,omitempty"`
}
