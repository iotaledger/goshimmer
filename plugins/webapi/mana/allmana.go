package mana

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	access, err := manaPlugin.GetManaMap(mana.AccessMana, mana.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAllManaResponse{
			Error: err.Error(),
		})
	}
	consensus, err := manaPlugin.GetManaMap(mana.ConsensusMana, mana.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAllManaResponse{
			Error: err.Error(),
		})
	}
	return c.JSON(http.StatusOK, GetAllManaResponse{
		Access:    access.ToNodeStrList(),
		Consensus: consensus.ToNodeStrList(),
	})
}

// GetAllManaResponse is the request to a getAllManaHandler request.
type GetAllManaResponse struct {
	Access    []mana.NodeStr `json:"access"`
	Consensus []mana.NodeStr `json:"consensus"`
	Error     string         `json:"error,omitempty"`
}
