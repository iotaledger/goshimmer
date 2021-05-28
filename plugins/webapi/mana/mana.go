package mana

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// getManaHandler handles the request.
func getManaHandler(c echo.Context) error {
	var request jsonmodels2.GetManaRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.GetManaResponse{Error: err.Error()})
	}
	ID, err := mana.IDFromStr(request.NodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.GetManaResponse{Error: err.Error()})
	}
	if request.NodeID == "" {
		ID = local.GetInstance().ID()
	}
	t := time.Now()
	accessMana, tAccess, err := manaPlugin.GetAccessMana(ID, t)
	if err != nil {
		if errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) {
			accessMana = 0
			tAccess = t
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels2.GetManaResponse{Error: err.Error()})
		}
	}
	consensusMana, tConsensus, err := manaPlugin.GetConsensusMana(ID, t)
	if err != nil {
		if errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) {
			consensusMana = 0
			tConsensus = t
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels2.GetManaResponse{Error: err.Error()})
		}
	}

	return c.JSON(http.StatusOK, jsonmodels2.GetManaResponse{
		ShortNodeID:        ID.String(),
		NodeID:             base58.Encode(ID.Bytes()),
		Access:             accessMana,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
