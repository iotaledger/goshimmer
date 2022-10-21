package mana

import (
	"net/http"
	"sort"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
)

func getOnlineAccessHandler(c echo.Context) error {
	return getOnlineHandler(c, manamodels.AccessMana)
}

func getOnlineConsensusHandler(c echo.Context) error {
	return getOnlineHandler(c, manamodels.ConsensusMana)
}

// getOnlineHandler handles the request.
func getOnlineHandler(c echo.Context, manaType manamodels.Type) error {
	manaMap, t, err := deps.Protocol.Engine().ManaTracker.GetManaMap(manaType)
	if err != nil {
		return c.JSON(http.StatusNotFound, jsonmodels.GetOnlineResponse{Error: err.Error()})
	}
	knownPeers := deps.Discovery.GetVerifiedPeers()
	resp := make([]jsonmodels.OnlineIssuerStr, 0)
	for _, knownPeer := range append(lo.Map(knownPeers, func(p *peer.Peer) identity.ID { return p.ID() }), deps.Local.ID()) {
		manaValue, exists := manaMap[knownPeer]
		if !exists {
			continue
		}

		resp = append(resp, jsonmodels.OnlineIssuerStr{
			ShortID: knownPeer.String(),
			ID:      base58.Encode(lo.PanicOnErr(knownPeer.Bytes())),
			Mana:    manaValue,
		})
	}
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Mana > resp[j].Mana
	})
	for rank, onlineIssuer := range resp {
		onlineIssuer.OnlineRank = rank + 1
	}

	return c.JSON(http.StatusOK, jsonmodels.GetOnlineResponse{
		Online:    resp,
		Timestamp: t.Unix(),
	})
}
