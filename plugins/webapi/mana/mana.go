package mana

import (
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/blocklayer"
)

// getManaHandler handles the request.
func getManaHandler(c echo.Context) error {
	var request jsonmodels.GetManaRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
	}
	ID, err := manamodels.IDFromStr(request.NodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
	}
	if request.NodeID == "" {
		ID = deps.Local.ID()
	}
	t := time.Now()
	accessMana, tAccess, err := manaPlugin.GetAccessMana(ID, t)
	if err != nil {
		if errors.Is(err, manamodels.ErrNodeNotFoundInBaseManaVector) {
			accessMana = 0
			tAccess = t
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}
	consensusMana, tConsensus, err := manaPlugin.GetConsensusMana(ID, t)
	if err != nil {
		if errors.Is(err, manamodels.ErrNodeNotFoundInBaseManaVector) {
			consensusMana = 0
			tConsensus = t
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}

	return c.JSON(http.StatusOK, jsonmodels.GetManaResponse{
		ShortNodeID:        ID.String(),
		NodeID:             base58.Encode(ID.Bytes()),
		Access:             accessMana,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
