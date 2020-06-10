package autopeering

import (
	"net"
	"net/http"
	"strconv"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// PluginName is the name of the web API autopeering endpoint plugin.
const PluginName = "WebAPI autopeering Endpoint"

// Plugin is the plugin instance of the web API autopeering endpoint plugin.
var Plugin = node.NewPlugin(PluginName, node.Enabled, configure)

func configure(plugin *node.Plugin) {
	webapi.Server.GET("autopeering/neighbors", getNeighbors)
}

// getNeighbors returns the chosen and accepted neighbors of the node
func getNeighbors(c echo.Context) error {

	var chosen []Neighbor
	var accepted []Neighbor
	var knownPeers []Neighbor

	if c.QueryParam("known") == "1" {
		for _, p := range autopeering.Discovery().GetVerifiedPeers() {
			knownPeers = append(knownPeers, createNeighborFromPeer(p))
		}
	}

	for _, p := range autopeering.Selection().GetOutgoingNeighbors() {
		chosen = append(chosen, createNeighborFromPeer(p))
	}
	for _, p := range autopeering.Selection().GetIncomingNeighbors() {
		accepted = append(accepted, createNeighborFromPeer(p))
	}

	return c.JSON(http.StatusOK, Response{KnownPeers: knownPeers, Chosen: chosen, Accepted: accepted})
}

func createNeighborFromPeer(p *peer.Peer) Neighbor {
	n := Neighbor{
		ID:        p.ID().String(),
		PublicKey: p.PublicKey().String(),
	}
	n.Services = getServices(p)

	return n
}

// Response contains information of the autopeering.
type Response struct {
	KnownPeers []Neighbor `json:"known,omitempty"`
	Chosen     []Neighbor `json:"chosen"`
	Accepted   []Neighbor `json:"accepted"`
	Error      string     `json:"error,omitempty"`
}

// Neighbor contains information of a neighbor peer.
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
	var services []peerService

	host := p.IP().String()
	peeringService := p.Services().Get(service.PeeringKey)
	if peeringService != nil {
		services = append(services, peerService{
			ID:      "peering",
			Address: net.JoinHostPort(host, strconv.Itoa(peeringService.Port())),
		})
	}

	gossipService := p.Services().Get(service.GossipKey)
	if gossipService != nil {
		services = append(services, peerService{
			ID:      "gossip",
			Address: net.JoinHostPort(host, strconv.Itoa(gossipService.Port())),
		})
	}

	fpcService := p.Services().Get(service.FPCKey)
	if fpcService != nil {
		services = append(services, peerService{
			ID:      "FPC",
			Address: net.JoinHostPort(host, strconv.Itoa(fpcService.Port())),
		})
	}

	return services
}
