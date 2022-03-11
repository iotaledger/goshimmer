package mana

import (
	"net/http"

	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// Handler handles the request.
func allowedManaPledgeHandler(c echo.Context) error {
	access := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	var accessNodes []string
	access.Allowed.ForEach(func(element identity.ID) {
		accessNodes = append(accessNodes, base58.Encode(element.Bytes()))
	})
	if len(accessNodes) == 0 {
		return c.JSON(http.StatusNotFound, jsonmodels.AllowedManaPledgeResponse{Error: "No access mana pledge IDs are accepted"})
	}

	consensus := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)
	var consensusNodes []string
	consensus.Allowed.ForEach(func(element identity.ID) {
		consensusNodes = append(consensusNodes, base58.Encode(element.Bytes()))
	})
	if len(consensusNodes) == 0 {
		return c.JSON(http.StatusNotFound, jsonmodels.AllowedManaPledgeResponse{Error: "No consensus mana pledge IDs are accepted"})
	}

	return c.JSON(http.StatusOK, jsonmodels.AllowedManaPledgeResponse{
		Access: jsonmodels.AllowedPledge{
			IsFilterEnabled: access.IsFilterEnabled,
			Allowed:         accessNodes,
		},
		Consensus: jsonmodels.AllowedPledge{
			IsFilterEnabled: consensus.IsFilterEnabled,
			Allowed:         consensusNodes,
		},
	})
}
