package allowedmanapledge

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

// Handler handles the request.
func Handler(c echo.Context) error {
	access := manaPlugin.GetAllowedAccessPledgeNodes(mana.AccessMana)
	var accessNodes []string
	for ID := range access.Allowed {
		accessNodes = append(accessNodes, ID.String())
	}
	consensus := manaPlugin.GetAllowedAccessPledgeNodes(mana.ConsensusMana)
	var consensusNodes []string
	for ID := range consensus.Allowed {
		consensusNodes = append(consensusNodes, ID.String())
	}
	return c.JSON(http.StatusOK, Response{
		Access: AllowedPledge{
			IsFilterEnabled: access.IsFilterEnabled,
			Allowed:         accessNodes,
		},
		Consensus: AllowedPledge{
			IsFilterEnabled: consensus.IsFilterEnabled,
			Allowed:         consensusNodes,
		},
	})
}

// Response is the http response.
type Response struct {
	Access    AllowedPledge `json:"accessMana"`
	Consensus AllowedPledge `json:"consensusMana"`
	Error     string        `json:"error,omitempty"`
}

// AllowedPledge represents the nodes that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool     `json:"isFilterEnabled"`
	Allowed         []string `json:"allowed,omitempty"`
}
