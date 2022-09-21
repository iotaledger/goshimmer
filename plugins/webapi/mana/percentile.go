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

// getPercentileHandler handles the request.
func getPercentileHandler(c echo.Context) error {
	var request jsonmodels.GetPercentileRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	ID, err := manamodels.IDFromStr(request.NodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	if request.NodeID == "" {
		ID = deps.Local.ID()
	}
	t := time.Now()
	access, tAccess, err := manaPlugin.GetManaMap(manamodels.AccessMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	accessPercentile, err := access.GetPercentile(ID)
	if err != nil {
		if errors.Is(err, manamodels.ErrNodeNotFoundInBaseManaVector) {
			accessPercentile = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}
	consensus, tConsensus, err := manaPlugin.GetManaMap(manamodels.ConsensusMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	consensusPercentile, err := consensus.GetPercentile(ID)
	if err != nil {
		if errors.Is(err, manamodels.ErrNodeNotFoundInBaseManaVector) {
			consensusPercentile = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}
	return c.JSON(http.StatusOK, jsonmodels.GetPercentileResponse{
		ShortNodeID:        ID.String(),
		NodeID:             base58.Encode(ID.Bytes()),
		Access:             accessPercentile,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusPercentile,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
