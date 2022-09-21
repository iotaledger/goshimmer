package mana

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/blocklayer"
)

// getNHighestAccessHandler handles a /mana/access/nhighest request.
func getNHighestAccessHandler(c echo.Context) error {
	return nHighestHandler(c, manamodels.AccessMana)
}

// getNHighestConsensusHandler handles a /mana/consensus/nhighest request.
func getNHighestConsensusHandler(c echo.Context) error {
	return nHighestHandler(c, manamodels.ConsensusMana)
}

// nHighestHandler handles the request.
func nHighestHandler(c echo.Context, manaType manamodels.Type) error {
	number, err := strconv.ParseUint(c.QueryParam("number"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetNHighestResponse{Error: err.Error()})
	}
	highestNodes, t, err := manaPlugin.GetHighestManaNodes(manaType, uint(number))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetNHighestResponse{Error: err.Error()})
	}
	var res []manamodels.IssuerStr
	for _, n := range highestNodes {
		res = append(res, n.ToIssuerStr())
	}
	return c.JSON(http.StatusOK, jsonmodels.GetNHighestResponse{
		Nodes:     res,
		Timestamp: t.Unix(),
	})
}
