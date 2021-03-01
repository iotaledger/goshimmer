package mana

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

// getManaHandler handles the request.
func getManaHandler(c echo.Context) error {
	var request GetManaRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, GetManaResponse{Error: err.Error()})
	}
	ID, err := mana.IDFromStr(request.NodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetManaResponse{Error: err.Error()})
	}
	emptyID := identity.ID{}
	if ID == emptyID {
		ID = local.GetInstance().ID()
	}
	t := time.Now()
	accessMana, tAccess, err := manaPlugin.GetAccessMana(ID, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetManaResponse{Error: err.Error()})
	}
	consensusMana, tConsensus, err := manaPlugin.GetConsensusMana(ID, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetManaResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, GetManaResponse{
		ShortNodeID:        ID.String(),
		NodeID:             base58.Encode(ID.Bytes()),
		Access:             accessMana,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}

// GetManaRequest is the request for get mana.
type GetManaRequest struct {
	NodeID string `json:"nodeID"`
}

// GetManaResponse defines the response for get mana.
type GetManaResponse struct {
	Error              string  `json:"error,omitempty"`
	ShortNodeID        string  `json:"shortNodeID"`
	NodeID             string  `json:"nodeID"`
	Access             float64 `json:"access"`
	AccessTimestamp    int64   `json:"accessTimestamp"`
	Consensus          float64 `json:"consensus"`
	ConsensusTimestamp int64   `json:"consensusTimestamp"`
}
