package all

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func Handler(c echo.Context) error {
	access := manaPlugin.GetManaMap(mana.AccessMana).ToNodeStrList()
	consensus := manaPlugin.GetManaMap(mana.ConsensusMana).ToNodeStrList()
	return c.JSON(http.StatusOK, Response{
		Access:    access,
		Consensus: consensus,
	})
}

// Response is the request response.
type Response struct {
	Access    []mana.NodeStr `json:"access"`
	Consensus []mana.NodeStr `json:"consensus"`
}
