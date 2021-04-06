package value

import (
	"net/http"

	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

// Handler handles the request.
func allowedManaPledgeHandler(c echo.Context) error {
	access := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	var accessNodes []string
	access.Allowed.ForEach(func(element interface{}) {
		accessNodes = append(accessNodes, base58.Encode(element.(identity.ID).Bytes()))
	})
	if len(accessNodes) == 0 {
		return c.JSON(http.StatusNotFound, value.AllowedManaPledgeResponse{Error: "No access mana pledge IDs are accepted"})
	}

	consensus := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)
	var consensusNodes []string
	consensus.Allowed.ForEach(func(element interface{}) {
		consensusNodes = append(consensusNodes, base58.Encode(element.(identity.ID).Bytes()))
	})
	if len(consensusNodes) == 0 {
		return c.JSON(http.StatusNotFound, value.AllowedManaPledgeResponse{Error: "No consensus mana pledge IDs are accepted"})
	}

	return c.JSON(http.StatusOK, value.AllowedManaPledgeResponse{
		Access: value.AllowedPledge{
			IsFilterEnabled: access.IsFilterEnabled,
			Allowed:         accessNodes,
		},
		Consensus: value.AllowedPledge{
			IsFilterEnabled: consensus.IsFilterEnabled,
			Allowed:         consensusNodes,
		},
	})
}
