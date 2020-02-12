package getNeighbors

import (
	"encoding/base64"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI getNeighbors Endpoint", node.Enabled, configure)

func configure(plugin *node.Plugin) {
	webapi.Server.GET("getNeighbors", getNeighbors)
}

// getNeighbors returns the chosen and accepted neighbors of the node
func getNeighbors(c echo.Context) error {

	chosen := []Neighbor{}
	accepted := []Neighbor{}
	knownPeers := []Neighbor{}

	if autopeering.Selection == nil {
		return c.JSON(http.StatusNotImplemented, Response{Error: "Neighbor Selection is not enabled"})
	}

	if autopeering.Discovery == nil {
		return c.JSON(http.StatusNotImplemented, Response{Error: "Neighbor Discovery is not enabled"})
	}

	if c.QueryParam("known") == "1" {
		for _, peer := range autopeering.Discovery.GetVerifiedPeers() {
			n := Neighbor{
				ID:        peer.ID().String(),
				PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey()),
			}
			n.Services = getServices(peer)
			knownPeers = append(knownPeers, n)
		}
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

	return c.JSON(http.StatusOK, Response{KnownPeers: knownPeers, Chosen: chosen, Accepted: accepted})
}

type Response struct {
	KnownPeers []Neighbor `json:"known,omitempty"`
	Chosen     []Neighbor `json:"chosen"`
	Accepted   []Neighbor `json:"accepted"`
	Error      string     `json:"error,omitempty"`
}

type Neighbor struct {
	ID        string        `json:"id"`        // comparable node identifier
	PublicKey string        `json:"publicKey"` // public key used to verify signatures
	Services  []peerService `json:"services,omitempty"`
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
