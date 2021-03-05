package mana

import (
	"net/http"
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	t := time.Now()
	access, tAccess, err := manaPlugin.GetManaMap(mana.AccessMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAllManaResponse{
			Error: err.Error(),
		})
	}
	accessList := access.ToNodeStrList()
	sort.Slice(accessList[:], func(i, j int) bool {
		return accessList[i].Mana > accessList[j].Mana
	})
	consensus, tConsensus, err := manaPlugin.GetManaMap(mana.ConsensusMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAllManaResponse{
			Error: err.Error(),
		})
	}
	consensusList := consensus.ToNodeStrList()
	sort.Slice(consensusList[:], func(i, j int) bool {
		return consensusList[i].Mana > consensusList[j].Mana
	})
	return c.JSON(http.StatusOK, GetAllManaResponse{
		Access:             accessList,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusList,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}

// GetAllManaResponse is the request to a getAllManaHandler request.
type GetAllManaResponse struct {
	Access             []mana.NodeStr `json:"access"`
	AccessTimestamp    int64          `json:"accessTimestamp"`
	Consensus          []mana.NodeStr `json:"consensus"`
	ConsensusTimestamp int64          `json:"consensusTimestamp"`
	Error              string         `json:"error,omitempty"`
}
