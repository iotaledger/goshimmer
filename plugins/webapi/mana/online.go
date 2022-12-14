package mana

import (
	"net/http"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
)

func getOnlineAccessHandler(c echo.Context) error {
	resp := make([]*jsonmodels.OnlineIssuerStr, 0)
	manaMap := deps.Protocol.Engine().ManaTracker.ManaByIDs()
	var knownPeers *set.AdvancedSet[identity.ID]
	if deps.Discovery != nil {
		knownPeers = set.NewAdvancedSet[identity.ID](lo.Map(deps.Discovery.GetVerifiedPeers(), func(p *peer.Peer) identity.ID {
			return p.ID()
		})...)
	}

	for p, manaValue := range manaMap {
		if knownPeers != nil && !knownPeers.Has(p) && p != deps.Local.ID() {
			continue
		}

		resp = append(resp, &jsonmodels.OnlineIssuerStr{
			ShortID: p.String(),
			ID:      p.EncodeBase58(),
			Mana:    manaValue,
		})
	}

	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Mana > resp[j].Mana || (resp[i].Mana == resp[j].Mana && resp[i].ID > resp[j].ID)
	})
	for rank, onlineIssuer := range resp {
		onlineIssuer.OnlineRank = rank + 1
	}

	return c.JSON(http.StatusOK, jsonmodels.GetOnlineResponse{
		Online:    resp,
		Timestamp: time.Now().Unix(),
	})
}

func getOnlineConsensusHandler(c echo.Context) error {
	resp := make([]*jsonmodels.OnlineIssuerStr, 0)
	manaMap := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Validators().Weights.Map())
	for p, manaValue := range manaMap {
		resp = append(resp, &jsonmodels.OnlineIssuerStr{
			ShortID: p.String(),
			ID:      p.EncodeBase58(),
			Mana:    manaValue,
		})
	}

	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Mana > resp[j].Mana || resp[i].ID > resp[j].ID
	})
	for rank, onlineIssuer := range resp {
		onlineIssuer.OnlineRank = rank + 1
	}

	return c.JSON(http.StatusOK, jsonmodels.GetOnlineResponse{
		Online:    resp,
		Timestamp: time.Now().Unix(),
	})
}
