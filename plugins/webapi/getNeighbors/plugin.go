package getNeighbors

import (
	"encoding/base64"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI getNeighbors Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-getNeighbors")
	webapi.Server.GET("getNeighbors", getNeighbors)
}

// getNeighbors returns the chosen and accepted neighbors of the node
func getNeighbors(c echo.Context) error {

	chosen := []Neighbor{}
	accepted := []Neighbor{}

	if autopeering.Selection == nil {
		return requestFailed(c, "Neighbor Selection is not enabled")
	}

	for _, peer := range autopeering.Selection.GetOutgoingNeighbors() {

		n := Neighbor{
			ID:        peer.ID().String(),
			PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey()),
		}
		n.Services = getServices(peer)
		chosen = append(chosen, n)
	}
	for _, peer := range autopeering.Selection.GetIncomingNeighbors() {
		n := Neighbor{
			ID:        peer.ID().String(),
			PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey()),
		}
		n.Services = getServices(peer)
		accepted = append(accepted, n)
	}
	return requestSuccessful(c, chosen, accepted)

}

func requestSuccessful(c echo.Context, chosen, accepted []Neighbor) error {
	return c.JSON(http.StatusOK, Response{
		Chosen:   chosen,
		Accepted: accepted,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, Response{
		Error: message,
	})
}

type Response struct {
	Chosen   []Neighbor `json:"chosen"`
	Accepted []Neighbor `json:"accepted"`
	Error    string     `json:"error,omitempty"`
}

type Neighbor struct {
	ID        string `json:"id"`        // comparable node identifier
	PublicKey string `json:"publicKey"` // public key used to verify signatures
	Services  []peerService
}

type peerService struct {
	ID      string `json:"id"`      // ID of the service
	Address string `json:"address"` // network address of the service
}

func getServices(p *peer.Peer) []peerService {
	services := []peerService{}
	peeringService := p.Services().Get(service.PeeringKey)
	if peeringService != nil {
		services = append(services, peerService{
			ID:      "peering",
			Address: peeringService.String(),
		})
	}

	gossipService := p.Services().Get(service.GossipKey)
	if gossipService != nil {
		services = append(services, peerService{
			ID:      "gossip",
			Address: gossipService.String(),
		})
	}

	fpcService := p.Services().Get(service.FPCKey)
	if fpcService != nil {
		services = append(services, peerService{
			ID:      "FPC",
			Address: fpcService.String(),
		})
	}

	return services
}
