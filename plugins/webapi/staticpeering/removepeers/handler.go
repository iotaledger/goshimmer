package removepeers

import (
	"net/http"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler removes peers
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var peerIDs []identity.ID
	for _, peerIDStr := range request.Peers {
		bytes, err := base58.Decode(peerIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}
		pubKey, _, err := ed25519.PublicKeyFromBytes(bytes)
		peerID := identity.NewID(pubKey)
		if err != nil {
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}
		peerIDs = append(peerIDs, peerID)
	}

	var removed = 0
	for _, peerID := range peerIDs {
		err := gossip.Manager().DropNeighbor(peerID)
		if err != nil {
			log.Errorf("%w: error dropping peer: %s", err)
		} else {
			log.Infof("successfully removed peer %s", peerID)
			removed++
		}
	}
	return c.JSON(http.StatusOK, Response{Removed: removed})
}

// Response represents the removePeers api response.
type Response struct {
	Removed int    `json:"removed"`
	Error   string `json:"error,omitempty"`
}

// Request represents the removePeers api request body.
type Request struct {
	Peers []string `json:"peers"`
}
