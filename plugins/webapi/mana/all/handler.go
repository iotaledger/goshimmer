package all

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func Handler(c echo.Context) error {
	access := nodeMapToList(manaPlugin.GetManaMap(mana.AccessMana))
	consensus := nodeMapToList(manaPlugin.GetManaMap(mana.ConsensusMana))
	return c.JSON(http.StatusOK, Response{
		Access:    access,
		Consensus: consensus,
	})
}

func nodeMapToList(nodeMap mana.NodeMap) []NodeMap {
	var list []NodeMap
	for ID, val := range nodeMap {
		m := make(NodeMap)
		m[ID.String()] = val
		list = append(list, m)
	}
	return list
}

// Response is the request response.
type Response struct {
	Access    []NodeMap `json:"access"`
	Consensus []NodeMap `json:"consensus"`
}

// NodeMap is a the mana of a node.
type NodeMap map[string]float64
