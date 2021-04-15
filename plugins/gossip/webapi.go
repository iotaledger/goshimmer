package gossip

import (
	"net/http"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/gossip"
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
		log.Errorw("Failed to parse neighbors from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(xerrors.Errorf("Invalid add neighbors request: %w", err)),
		)
	}
	var addErr error
	for _, neighbor := range neighbors {
		if err := mgr.AddOutbound(neighbor, gossip.NeighborsGroupManual); err != nil {
			log.Errorw(
				"Can't add neighbor from the HTTP request to gossip manually, skipping that neighbor",
				"err", err, "neighbor", neighbor,
			)
			if addErr == nil {
				addErr = xerrors.Errorf("Gossip failed to add neighbor %+v: %w", neighbor, err)
			}
		}
	}
	if addErr != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(addErr))
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

type neighborToDrop struct {
	PublicKey string `json:"publicKey"`
}

func dropNeighborsHandler(c echo.Context) error {
	var neighborsToDrop []*neighborToDrop
	if err := webapi.ParseJSONRequest(c, &neighborsToDrop); err != nil {
		log.Errorw("Failed to parse neighbors for drop from the request", "err", err)
		return c.JSON(
			http.StatusBadRequest,
			jsonmodels.NewErrorResponse(xerrors.Errorf("Invalid drop neighbors request: %w", err)),
		)
	}
	var dropErr error
	for _, ntd := range neighborsToDrop {
		if err := dropNeighbor(ntd); err != nil {
			log.Errorw(
				"Can't drop the neighbor from the HTTP request, skipping that neighbor",
				"err", err, "neighbor", ntd,
			)
			if dropErr == nil {
				dropErr = xerrors.Errorf("Gossip failed to drop neighbor %+v: %w", ntd, err)
			}
		}
	}
	if dropErr != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(dropErr))
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func dropNeighbor(ntd *neighborToDrop) error {
	publicKey, err := ed25519.PublicKeyFromString(ntd.PublicKey)
	if err != nil {
		return xerrors.Errorf("failed to parse public key from HTTP request: %w", err)
	}
	id := identity.NewID(publicKey)
	if err := mgr.DropNeighbor(id, gossip.NeighborsGroupManual); err != nil {
		return xerrors.Errorf("gossip layer failed to drop the neighbor: %w", err)
	}
	return nil
}
