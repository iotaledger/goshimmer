package nhighest

import (
	"net/http"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func Handler(c echo.Context, manaType mana.Type) error {
	number, err := strconv.ParseUint(c.QueryParam("number"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	highestNodes := manaPlugin.GetHighestManaNodes(manaType, uint(number))
	var res []mana.NodeStr
	for _, n := range highestNodes {
		res = append(res, n.ToNodeStr())
	}
	return c.JSON(http.StatusOK, Response{
		Nodes: res,
	})
}

// Response is the response.
type Response struct {
	Error string `json:"error,omitempty"`
	Nodes []mana.NodeStr
}
