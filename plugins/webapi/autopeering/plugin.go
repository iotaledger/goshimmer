package autopeering

import (
	"encoding/base64"
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
				PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey().Bytes()),
			}
			n.Services = getServices(peer)
			knownPeers = append(knownPeers, n)
		}
	}

	for _, peer := range autopeering.Selection.GetOutgoingNeighbors() {
		n := Neighbor{
			ID:        peer.ID().String(),
			PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey().Bytes()),
		}
		n.Services = getServices(peer)
		chosen = append(chosen, n)
	}
	for _, peer := range autopeering.Selection.GetIncomingNeighbors() {
		n := Neighbor{
			ID:        peer.ID().String(),
			PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey().Bytes()),
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
