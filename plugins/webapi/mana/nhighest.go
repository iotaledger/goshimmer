package mana

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	manamodels2 "github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana2/manamodels"
)

// getNHighestAccessHandler handles a /mana/access/nhighest request.
func getNHighestAccessHandler(c echo.Context) error {
	return nHighestHandler(c, manamodels2.AccessMana)
}

// getNHighestConsensusHandler handles a /mana/consensus/nhighest request.
func getNHighestConsensusHandler(c echo.Context) error {
	return nHighestHandler(c, manamodels2.ConsensusMana)
}

// nHighestHandler handles the request.
func nHighestHandler(c echo.Context, manaType manamodels2.Type) error {
	number, err := strconv.ParseUint(c.QueryParam("number"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetNHighestResponse{Error: err.Error()})
	}
	var manaMap map[identity.ID]int64
	if manaType == manamodels2.AccessMana {
		manaMap = deps.Protocol.Engine().ThroughputQuota.ManaByIDs()
	} else {
		manaMap = lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())
	}
	highestNodes, t, err := manamodels2.GetHighestManaIssuers(uint(number), manaMap)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetNHighestResponse{Error: err.Error()})
	}
	var res []manamodels2.IssuerStr
	for _, n := range highestNodes {
		res = append(res, n.ToIssuerStr())
	}
	return c.JSON(http.StatusOK, jsonmodels.GetNHighestResponse{
		Issuers:   res,
		Timestamp: t.Unix(),
	})
}
