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
	webapi.Server().GET("manualpeering/peers", getPeersHandler)
}

func addPeersHandler(c echo.Context) error {
	var peers []*peer.Peer
	if err := webapi.ParseJSONRequest(c, &peers); err != nil {
		log().Errorw("Failed to parse peers from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid add peers request")),
		)
	}
	Manager().AddPeers(peers)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

type peerToRemove struct {
	PublicKey string `json:"publicKey"`
}

func removePeersHandler(c echo.Context) error {
	var peersToRemove []*peerToRemove
	if err := webapi.ParseJSONRequest(c, &peersToRemove); err != nil {
		log().Errorw("Failed to parse peers to remove from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(errors.Wrap(err, "Invalid remove peers request")),
		)
	}
	if err := removePeers(peersToRemove); err != nil {
		log().Errorw(
			"Can't remove some of the peers from the HTTP request",
			"err", err,
		)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func removePeers(ntds []*peerToRemove) error {
	keys := make([]ed25519.PublicKey, len(ntds))
	for i, ntd := range ntds {
		publicKey, err := ed25519.PublicKeyFromString(ntd.PublicKey)
		if err != nil {
			return errors.Wrapf(err, "failed to parse public key %s from HTTP request", publicKey)
		}
		keys[i] = publicKey
	}
	Manager().RemovePeers(keys)
	return nil
}

func getPeersHandler(c echo.Context) error {
	peers := Manager().GetPeers()
	return c.JSON(http.StatusOK, peers)
}
