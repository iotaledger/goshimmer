package percentile

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	ID, err := mana.IDFromStr(request.Node)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	emptyID := identity.ID{}
	if ID == emptyID {
		ID = local.GetInstance().ID()
	}
	access, err := manaPlugin.GetManaMap(mana.AccessMana).GetPercentile(ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	consensus, err := manaPlugin.GetManaMap(mana.ConsensusMana).GetPercentile(ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{
		Node:      request.Node,
		Access:    access,
		Consensus: consensus,
	})
}

// Request is the request.
type Request struct {
	Node string `json:"node"`
}

// Response is the response.
type Response struct {
	Error     string  `json:"error,omitempty"`
	Node      string  `json:"node"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
}
