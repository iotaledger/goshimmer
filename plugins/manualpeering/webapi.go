package manualpeering

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/manualpeering"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// RouteManualPeers defines the HTTP path for manualpeering peers endpoint.
const RouteManualPeers = "manualpeering/peers"

func configureWebAPI() {
	deps.Server.POST(RouteManualPeers, addPeersHandler)
	deps.Server.DELETE(RouteManualPeers, removePeersHandler)
	deps.Server.GET(RouteManualPeers, getPeersHandler)
}

/*
An example of the HTTP JSON request:
[
    {
        "publicKey": "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
        "address": "172.19.0.3:14666"
    }
].
*/
func addPeersHandler(c echo.Context) error {
	var peers []*manualpeering.KnownPeerToAdd
	if err := webapi.ParseJSONRequest(c, &peers); err != nil {
		Plugin.Logger().Errorw("Failed to parse peers from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid add peers request")),
		)
	}
	if err := deps.ManualPeeringMgr.AddPeer(peers...); err != nil {
		Plugin.Logger().Errorw(
			"Can't add some of the peers from the HTTP request to manualpeering manager",
			"err", err,
		)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	return c.NoContent(http.StatusNoContent)
}

/*
An example of the HTTP JSON request:
[
    {
        "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
    }
].
*/
func removePeersHandler(c echo.Context) error {
	var peersToRemove []*jsonmodels.PeerToRemove
	if err := webapi.ParseJSONRequest(c, &peersToRemove); err != nil {
		Plugin.Logger().Errorw("Failed to parse peers to remove from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid remove peers request")),
		)
	}
	if err := removePeers(peersToRemove); err != nil {
		Plugin.Logger().Errorw(
			"Can't remove some of the peers from the HTTP request",
			"err", err,
		)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	return c.NoContent(http.StatusNoContent)
}

func removePeers(peers []*jsonmodels.PeerToRemove) error {
	keys := make([]ed25519.PublicKey, len(peers))
	for i, p := range peers {
		keys[i] = p.PublicKey
	}
	if err := deps.ManualPeeringMgr.RemovePeer(keys...); err != nil {
		return errors.Wrap(err, "manualpeering manager failed to remove some peers")
	}
	return nil
}

func getPeersHandler(c echo.Context) error {
	conf := &manualpeering.GetPeersConfig{}
	if err := webapi.ParseJSONRequest(c, conf); err != nil {
		Plugin.Logger().Errorw("Failed to parse get peers config from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid get peers request")),
		)
	}
	peers := deps.ManualPeeringMgr.GetPeers(conf.ToOptions()...)
	return c.JSON(http.StatusOK, peers)
}
