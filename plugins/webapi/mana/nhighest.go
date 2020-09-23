package mana

import (
	"net/http"
	"strconv"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getNHighestAccess handles a /mana/access/nhighest request.
func getNHighestAccess(c echo.Context) error {
	return nhighest(c, manaPkg.AccessMana)
}

// getNHighestConsensus handles a /mana/consensus/nhighest request.
func getNHighestConsensus(c echo.Context) error {
	return nhighest(c, manaPkg.ConsensusMana)
}

// Handler handles the request.
func nhighest(c echo.Context, manaType manaPkg.Type) error {
	number, err := strconv.ParseUint(c.QueryParam("number"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetNHighestResponse{Error: err.Error()})
	}
	highestNodes := manaPlugin.GetHighestManaNodes(manaType, uint(number))
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
