package mana

import (
	"net/http"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// getAllManaHandler handles the request.
func getAllManaHandler(c echo.Context) error {
	access, err := manaPlugin.GetManaMap(manaPkg.AccessMana, manaPkg.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, getAllManaResponse{
			Error: err.Error(),
		})
	}
	consensus, err := manaPlugin.GetManaMap(manaPkg.ConsensusMana, manaPkg.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, getAllManaResponse{
			Error: err.Error(),
		})
	}
	return c.JSON(http.StatusOK, getAllManaResponse{
		Access:    access.ToNodeStrList(),
		Consensus: consensus.ToNodeStrList(),
	})
}

// getAllManaResponse is the request to a getAllManaHandler request.
type getAllManaResponse struct {
	Access    []manaPkg.NodeStr `json:"access"`
	Consensus []manaPkg.NodeStr `json:"consensus"`
	Error     string            `json:"error,omitempty"`
}
