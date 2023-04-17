package mana

import (
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/hive.go/lo"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	access := deps.Protocol.Engine().ThroughputQuota.BalanceByIDs()
	accessList := manamodels.IssuerMap(access).ToIssuerStrList()
	sort.Slice(accessList, func(i, j int) bool {
		return accessList[i].Mana > accessList[j].Mana
	})
	consensusList := manamodels.IssuerMap(lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())).ToIssuerStrList()
	sort.Slice(consensusList, func(i, j int) bool {
		return consensusList[i].Mana > consensusList[j].Mana
	})
	return c.JSON(http.StatusOK, jsonmodels.GetAllManaResponse{
		Access:             accessList,
		AccessTimestamp:    time.Now().Unix(),
		Consensus:          consensusList,
		ConsensusTimestamp: time.Now().Unix(),
	})
}
