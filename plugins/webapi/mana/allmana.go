package mana

import (
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/blocklayer"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	t := time.Now()
	access, tAccess, err := manaPlugin.GetManaMap(mana.AccessMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetAllManaResponse{
			Error: err.Error(),
		})
	}
	accessList := access.ToNodeStrList()
	sort.Slice(accessList, func(i, j int) bool {
		return accessList[i].Mana > accessList[j].Mana
	})
	consensus, tConsensus, err := manaPlugin.GetManaMap(mana.ConsensusMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetAllManaResponse{
			Error: err.Error(),
		})
	}
	consensusList := consensus.ToNodeStrList()
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
