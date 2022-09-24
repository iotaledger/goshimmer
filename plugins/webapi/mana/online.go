package mana

import (
	"net/http"
	"sort"

	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/congestioncontrol/icca/mana/manamodels"
)

func getOnlineAccessHandler(c echo.Context) error {
	return getOnlineHandler(c, manamodels.AccessMana)
}

func getOnlineConsensusHandler(c echo.Context) error {
	return getOnlineHandler(c, manamodels.ConsensusMana)
}

// getOnlineHandler handles the request.
func getOnlineHandler(c echo.Context, manaType manamodels.Type) error {
	manaMap, t, err := deps.Protocol.Instance().CongestionControl.GetManaMap(manaType)
	if err != nil {
		return c.JSON(http.StatusNotFound, jsonmodels.GetOnlineResponse{Error: err.Error()})
	}
	knownPeers := deps.Discovery.GetVerifiedPeers()
	resp := make([]jsonmodels.OnlineIssuerStr, 0)
	for _, knownPeer := range knownPeers {
		manaValue, exists := manaMap[knownPeer.ID()]
		if !exists {
			continue
		}

		resp = append(resp, jsonmodels.OnlineIssuerStr{
			ShortID: knownPeer.ID().String(),
			ID:      base58.Encode(knownPeer.ID().Bytes()),
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
