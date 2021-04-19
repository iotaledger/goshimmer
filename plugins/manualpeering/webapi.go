package manualpeering

import (
	"github.com/cockroachdb/errors"
	"net/http"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

func configureWebAPI() {
	webapi.Server().POST("gossip/neighbors", addNeighborsHandler)
	webapi.Server().DELETE("gossip/neighbors", dropNeighborsHandler)
}

func addNeighborsHandler(c echo.Context) error {
	var neighbors []*peer.Peer
	if err := webapi.ParseJSONRequest(c, &neighbors); err != nil {
		log().Errorw("Failed to parse neighbors from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(xerrors.Errorf("Invalid add neighbors request: %w", err)),
		)
	}
	Manager().AddPeers(neighbors)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

type neighborToDrop struct {
	PublicKey string `json:"publicKey"`
}

func dropNeighborsHandler(c echo.Context) error {
	var neighborsToDrop []*neighborToDrop
	if err := webapi.ParseJSONRequest(c, &neighborsToDrop); err != nil {
		log().Errorw("Failed to parse neighbors for drop from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(xerrors.Errorf("Invalid drop neighbors request: %w", err)),
		)
	}
	if err := dropNeighbors(neighborsToDrop); err != nil {
		log().Errorw(
			"Can't drop some of the neighbors from the HTTP request",
			"err", err,
		)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func dropNeighbors(ntds []*neighborToDrop) error {
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
