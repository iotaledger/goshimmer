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

// getPercentileHandler handles the request.
func getPercentileHandler(c echo.Context) error {
	var request jsonmodels2.GetPercentileRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.GetPercentileResponse{Error: err.Error()})
	}
	ID, err := mana.IDFromStr(request.NodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.GetPercentileResponse{Error: err.Error()})
	}
	if request.NodeID == "" {
		ID = local.GetInstance().ID()
	}
	t := time.Now()
	access, tAccess, err := manaPlugin.GetManaMap(mana.AccessMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.GetPercentileResponse{Error: err.Error()})
	}
	accessPercentile, err := access.GetPercentile(ID)
	if err != nil {
		if errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) {
			accessPercentile = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels2.GetManaResponse{Error: err.Error()})
		}
	}
	consensus, tConsensus, err := manaPlugin.GetManaMap(mana.ConsensusMana, t)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.GetPercentileResponse{Error: err.Error()})
	}
	consensusPercentile, err := consensus.GetPercentile(ID)
	if err != nil {
		if errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) {
			consensusPercentile = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels2.GetManaResponse{Error: err.Error()})
		}
	}
	return c.JSON(http.StatusOK, jsonmodels2.GetPercentileResponse{
		ShortNodeID:        ID.String(),
		NodeID:             base58.Encode(ID.Bytes()),
		Access:             accessPercentile,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusPercentile,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
