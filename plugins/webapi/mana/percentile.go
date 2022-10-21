package mana

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
	"github.com/iotaledger/hive.go/core/generics/lo"
)

// getPercentileHandler handles the request.
func getPercentileHandler(c echo.Context) error {
	var request jsonmodels.GetPercentileRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	ID, err := manamodels.IDFromStr(request.IssuerID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	if request.IssuerID == "" {
		ID = deps.Local.ID()
	}

	access, tAccess, err := deps.Protocol.Engine().ManaTracker.GetManaMap(manamodels.AccessMana)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	accessPercentile, err := access.GetPercentile(ID)
	if err != nil {
		if errors.Is(err, manamodels.ErrIssuerNotFoundInBaseManaVector) {
			accessPercentile = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}
	consensus, tConsensus, err := deps.Protocol.Engine().ManaTracker.GetManaMap(manamodels.ConsensusMana)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetPercentileResponse{Error: err.Error()})
	}
	consensusPercentile, err := consensus.GetPercentile(ID)
	if err != nil {
		if errors.Is(err, manamodels.ErrIssuerNotFoundInBaseManaVector) {
			consensusPercentile = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}
	return c.JSON(http.StatusOK, jsonmodels.GetPercentileResponse{
		ShortIssuerID:      ID.String(),
		IssuerID:           base58.Encode(lo.PanicOnErr(ID.Bytes())),
		Access:             accessPercentile,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusPercentile,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
