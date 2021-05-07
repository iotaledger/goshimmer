package manualpeering

import (
	"net/http"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

func configureWebAPI() {
	webapi.Server().POST("manualpeering/peers", addPeersHandler)
	webapi.Server().DELETE("manualpeering/peers", removePeersHandler)
	webapi.Server().GET("manualpeering/peers/known", getKnownPeersHandler)
	webapi.Server().GET("manualpeering/peers/connected", getConnectedPeersHandler)
}

/*
An example of the HTTP JSON request:
[
    {
        "publicKey": "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
        "ip": "172.19.0.3",
        "services": {
            "peering":{
                "network":"TCP",
                "port":14626
            },
            "gossip": {
                "network": "TCP",
                "port": 14666
            }
        }
    }
]
*/
func addPeersHandler(c echo.Context) error {
	var peers []*peer.Peer
	if err := webapi.ParseJSONRequest(c, &peers); err != nil {
		plugin.Logger().Errorw("Failed to parse peers from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid add peers request")),
		)
	}
	Manager().AddPeer(peers...)
	return c.NoContent(http.StatusNoContent)
}

// PeerToRemove holds the data that uniquely identifies the peer to be removed, e.g. public key or ID.
// Only public key is supported for now.
type PeerToRemove struct {
	PublicKey string `json:"publicKey"`
}

/*
An example of the HTTP JSON request:
[
    {
        "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
    }
]
*/
func removePeersHandler(c echo.Context) error {
	var peersToRemove []*PeerToRemove
	if err := webapi.ParseJSONRequest(c, &peersToRemove); err != nil {
		plugin.Logger().Errorw("Failed to parse peers to remove from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid remove peers request")),
		)
	}
	if err := removePeers(peersToRemove); err != nil {
		plugin.Logger().Errorw(
			"Can't remove some of the peers from the HTTP request",
			"err", err,
		)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	return c.NoContent(http.StatusNoContent)
}

func removePeers(ntds []*PeerToRemove) error {
	keys := make([]ed25519.PublicKey, len(ntds))
	for i, ntd := range ntds {
		publicKey, err := ed25519.PublicKeyFromString(ntd.PublicKey)
		if err != nil {
			return errors.Wrapf(err, "failed to parse public key %s from HTTP request", publicKey)
		}
		keys[i] = publicKey
	}
	Manager().RemovePeer(keys...)
	return nil
}

func getKnownPeersHandler(c echo.Context) error {
	peers := Manager().GetKnownPeers()
	return c.JSON(http.StatusOK, peers)
}

func getConnectedPeersHandler(c echo.Context) error {
	peers := Manager().GetConnectedPeers()
	return c.JSON(http.StatusOK, peers)
}
