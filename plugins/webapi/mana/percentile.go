package mana

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

// getPercentileHandler handles the request.
func getPercentileHandler(c echo.Context) error {
	var request GetPercentileRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, GetPercentileResponse{Error: err.Error()})
	}
	ID, err := mana.IDFromStr(request.NodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetPercentileResponse{Error: err.Error()})
	}
	emptyID := identity.ID{}
	if ID == emptyID {
		ID = local.GetInstance().ID()
	}
	access, err := manaPlugin.GetManaMap(mana.AccessMana, mana.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetPercentileResponse{Error: err.Error()})
	}
	accessPercentile, err := access.GetPercentile(ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetPercentileResponse{Error: err.Error()})
	}
	consensus, err := manaPlugin.GetManaMap(mana.ConsensusMana, mana.Mixed)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetPercentileResponse{Error: err.Error()})
	}
	consensusPercentile, err := consensus.GetPercentile(ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetPercentileResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, GetPercentileResponse{
		ShortNodeID: ID.String(),
		NodeID:      base58.Encode(ID.Bytes()),
		Access:      accessPercentile,
		Consensus:   consensusPercentile,
	})
}

// GetPercentileRequest is the request object of mana/percentile.
type GetPercentileRequest struct {
	NodeID string `json:"nodeID"`
}

// GetPercentileResponse holds info about the mana percentile(s) of a node.
type GetPercentileResponse struct {
	Error       string  `json:"error,omitempty"`
	ShortNodeID string  `json:"shortNodeID"`
	NodeID      string  `json:"nodeID"`
	Access      float64 `json:"access"`
	Consensus   float64 `json:"consensus"`
}
