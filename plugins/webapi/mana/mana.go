package mana

import (
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
)

// getManaHandler handles the request.
func getManaHandler(c echo.Context) error {
	var request jsonmodels.GetManaRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
	}

	IDstr := request.IssuerID
	if IDstr == "" {
		IDstr = deps.Local.ID().EncodeBase58()
	}

	ID, err := identity.DecodeIDBase58(IDstr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.GetManaResponse{Error: err.Error()})
	}

	accessMana, _ := deps.Protocol.Engine().ThroughputQuota.Balance(ID)
	consensusMana := lo.Return1(deps.Protocol.Engine().SybilProtection.Weights().Get(ID)).Value

	return c.JSON(http.StatusOK, jsonmodels.GetManaResponse{
		ShortIssuerID:      ID.String(),
		IssuerID:           base58.Encode(lo.PanicOnErr(ID.Bytes())),
		Access:             accessMana,
		AccessTimestamp:    time.Now().Unix(),
		Consensus:          consensusMana,
		ConsensusTimestamp: time.Now().Unix(),
	})
}
