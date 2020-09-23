package mana

import (
	"net/http"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func getAllMana(c echo.Context) error {
	access := manaPlugin.GetManaMap(manaPkg.AccessMana).ToNodeStrList()
	consensus := manaPlugin.GetManaMap(manaPkg.ConsensusMana).ToNodeStrList()
	return c.JSON(http.StatusOK, getAllManaResponse{
		Access:    access,
		Consensus: consensus,
	})
}

// getAllManaResponse is the request to a getAllMana request.
type getAllManaResponse struct {
	Access    []manaPkg.NodeStr `json:"access"`
	Consensus []manaPkg.NodeStr `json:"consensus"`
}
