package mana

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/mana/manamodels"
)

// getManaHandler handles the request.
func getManaHandler(c echo.Context) error {
	var request jsonmodels.GetManaRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
	}
	ID, err := manamodels.IDFromStr(request.IssuerID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
	}
	if request.IssuerID == "" {
		ID = deps.Local.ID()
	}

	accessMana, tAccess, err := deps.Protocol.Instance().CongestionControl.GetAccessMana(ID)
	if err != nil {
		if errors.Is(err, manamodels.ErrIssuerNotFoundInBaseManaVector) {
			accessMana = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}
	consensusMana, tConsensus, err := deps.Protocol.Instance().CongestionControl.GetConsensusMana(ID)
	if err != nil {
		if errors.Is(err, manamodels.ErrIssuerNotFoundInBaseManaVector) {
			consensusMana = 0
		} else {
			return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
		}
	}

	return c.JSON(http.StatusOK, jsonmodels.GetManaResponse{
		ShortIssuerID:      ID.String(),
		IssuerID:           base58.Encode(ID.Bytes()),
		Access:             accessMana,
		AccessTimestamp:    tAccess.Unix(),
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus.Unix(),
	})
}
