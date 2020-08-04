package addpeers

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

var (
	// ErrParsingNode is returned when the node cannot be parsed.
	ErrParsingNode = errors.New("cannot parse node")
)

// Handler adds peers statically
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	peers, err := parsePeers(request.Peers)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var added = 0
	for _, p := range peers {
		err := gossip.Manager().AddOutbound(p)
		if err != nil {
			log.Errorf("%w", err)
		} else {
			log.Infof("peered successfully: %s", p)
			added++
		}
	}
	return c.JSON(http.StatusCreated, Response{Added: added})
}

func parsePeers(peers []string) (result []*peer.Peer, err error) {
	for _, p := range peers {
		if p == "" {
			continue
		}

		parts := strings.Split(p, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: master node parts must be 2, is %d", ErrParsingNode, len(parts))
		}
		pubKey, err := base58.Decode(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid public key: %s", ErrParsingNode, err)
		}
		addr, err := net.ResolveUDPAddr("udp", parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: host cannot be resolved: %s", ErrParsingNode, err)
		}
		publicKey, _, err := ed25519.PublicKeyFromBytes(pubKey)
		if err != nil {
			return nil, err
		}

		services := service.New()
		services.Update(service.PeeringKey, addr.Network(), addr.Port)
		services.Update(service.GossipKey, addr.Network(), addr.Port)

		result = append(result, peer.NewPeer(identity.New(publicKey), addr.IP, services))
	}

	return result, nil
}

// Response represents the addPeers api response.
type Response struct {
	Added int    `json:"added"`
	Error string `json:"error,omitempty"`
}

// Request represents the addPeers api request body.
type Request struct {
	Peers []string `json:"peers"`
}
