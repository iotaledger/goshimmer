package allowedmanapledge

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

// Handler handles the request.
func Handler(c echo.Context) error {
	access := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	var accessNodes []string
	access.Allowed.ForEach(func(element interface{}) {
		accessNodes = append(accessNodes, base58.Encode(element.(identity.ID).Bytes()))
	})
	if len(accessNodes) == 0 {
		return c.JSON(http.StatusNotFound, Response{Error: "No access mana pledge IDs are accepted"})
	}

	consensus := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)
	var consensusNodes []string
	consensus.Allowed.ForEach(func(element interface{}) {
		consensusNodes = append(consensusNodes, base58.Encode(element.(identity.ID).Bytes()))
	})
	if len(consensusNodes) == 0 {
		return c.JSON(http.StatusNotFound, Response{Error: "No consensus mana pledge IDs are accepted"})
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
