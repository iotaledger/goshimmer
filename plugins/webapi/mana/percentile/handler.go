package percentile

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func Handler(c echo.Context) error {
	accessNodes := manaPlugin.GetManaMap(mana.AccessMana)
	access := getPercentile(accessNodes)
	consensusNodes := manaPlugin.GetManaMap(mana.ConsensusMana)
	consensus := getPercentile(consensusNodes)
	return c.JSON(http.StatusOK, Response{
		Access:    access,
		Consensus: consensus,
	})
}

func getPercentile(nodes mana.NodeMap) float64 {
	if len(nodes) == 0 {
		return 0
	}
	mine, ok := nodes[local.GetInstance().ID()]
	if !ok {
		return 0
	}
	nBelow := 0.0
	for _, val := range nodes {
		if val < mine {
			nBelow++
		}
	}

	return (nBelow / float64(len(nodes))) * 100
}

// Response is the response.
type Response struct {
	Error     string  `json:"error,omitempty"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
}
