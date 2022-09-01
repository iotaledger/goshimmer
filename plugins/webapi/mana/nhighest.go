package mana

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	mana2 "github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/blocklayer"
)

// getNHighestAccessHandler handles a /mana/access/nhighest request.
func getNHighestAccessHandler(c echo.Context) error {
	return nHighestHandler(c, mana2.AccessMana)
}

// getNHighestConsensusHandler handles a /mana/consensus/nhighest request.
func getNHighestConsensusHandler(c echo.Context) error {
	return nHighestHandler(c, mana2.ConsensusMana)
}

// nHighestHandler handles the request.
func nHighestHandler(c echo.Context, manaType mana2.Type) error {
	number, err := strconv.ParseUint(c.QueryParam("number"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetNHighestResponse{Error: err.Error()})
	}
	highestNodes, t, err := manaPlugin.GetHighestManaNodes(manaType, uint(number))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetNHighestResponse{Error: err.Error()})
	}
	var res []mana2.NodeStr
	for _, n := range highestNodes {
		res = append(res, n.ToNodeStr())
	}
	return c.JSON(http.StatusOK, jsonmodels.GetNHighestResponse{
		Nodes:     res,
		Timestamp: t.Unix(),
	})
}
