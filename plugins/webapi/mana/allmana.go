package mana

import (
	"net/http"
	"sort"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/mana/manamodels"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	access, tAccess, err := deps.Protocol.Engine().CongestionControl.GetManaMap(manamodels.AccessMana)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetAllManaResponse{
			Error: err.Error(),
		})
	}
	accessList := access.ToIssuerStrList()
	sort.Slice(accessList, func(i, j int) bool {
		return accessList[i].Mana > accessList[j].Mana
	})
	consensus, tConsensus, err := deps.Protocol.Engine().CongestionControl.GetManaMap(manamodels.ConsensusMana)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetAllManaResponse{
			Error: err.Error(),
		})
	}
	consensusList := consensus.ToIssuerStrList()
	sort.Slice(consensusList, func(i, j int) bool {
		return consensusList[i].Mana > consensusList[j].Mana
	})
	return c.JSON(http.StatusOK, jsonmodels.GetAllManaResponse{
		Access:             accessList,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusList,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
