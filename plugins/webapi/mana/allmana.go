package mana

import (
	"net/http"
	"sort"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	access, err := manaPlugin.GetManaMap(mana.AccessMana)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAllManaResponse{
			Error: err.Error(),
		})
	}
	accessList := access.ToNodeStrList()
	sort.Slice(accessList[:], func(i, j int) bool {
		return accessList[i].Mana > accessList[j].Mana
	})
	consensus, err := manaPlugin.GetManaMap(mana.ConsensusMana)
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
		Access:    accessList,
		Consensus: consensusList,
	})
}

// GetAllManaResponse is the request to a getAllManaHandler request.
type GetAllManaResponse struct {
	Access    []mana.NodeStr `json:"access"`
	Consensus []mana.NodeStr `json:"consensus"`
	Error     string         `json:"error,omitempty"`
}
