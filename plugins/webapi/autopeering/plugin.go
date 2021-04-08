package autopeering

import (
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

// PluginName is the name of the web API autopeering endpoint plugin.
const PluginName = "WebAPI autopeering Endpoint"

var (
	// plugin is the plugin instance of the web API autopeering endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

func configure(plugin *node.Plugin) {
	webapi.Server().GET("autopeering/neighbors", getNeighbors)
}

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

// getNeighbors returns the chosen and accepted neighbors of the node
func getNeighbors(c echo.Context) error {
	var chosen []jsonmodels.Neighbor
	var accepted []jsonmodels.Neighbor
	var knownPeers []jsonmodels.Neighbor

	if c.QueryParam("known") == "1" {
		for _, p := range discovery.Discovery().GetVerifiedPeers() {
			knownPeers = append(knownPeers, createNeighborFromPeer(p))
		}
	}

	for _, p := range autopeering.Selection().GetOutgoingNeighbors() {
		chosen = append(chosen, createNeighborFromPeer(p))
	}
	for _, p := range autopeering.Selection().GetIncomingNeighbors() {
		accepted = append(accepted, createNeighborFromPeer(p))
	}

	return c.JSON(http.StatusOK, jsonmodels.GetNeighborsResponse{KnownPeers: knownPeers, Chosen: chosen, Accepted: accepted})
}

func createNeighborFromPeer(p *peer.Peer) jsonmodels.Neighbor {
	n := jsonmodels.Neighbor{
		ID:        p.ID().String(),
		PublicKey: p.PublicKey().String(),
	}
	n.Services = getServices(p)

	return n
}

func getServices(p *peer.Peer) []jsonmodels.PeerService {
	var services []jsonmodels.PeerService

	host := p.IP().String()
	peeringService := p.Services().Get(service.PeeringKey)
	if peeringService != nil {
		services = append(services, jsonmodels.PeerService{
			ID:      "peering",
			Address: net.JoinHostPort(host, strconv.Itoa(peeringService.Port())),
		})
	}

	gossipService := p.Services().Get(service.GossipKey)
	if gossipService != nil {
		services = append(services, jsonmodels.PeerService{
			ID:      "gossip",
			Address: net.JoinHostPort(host, strconv.Itoa(gossipService.Port())),
		})
	}

	fpcService := p.Services().Get(service.FPCKey)
	if fpcService != nil {
		services = append(services, jsonmodels.PeerService{
			ID:      "FPC",
			Address: net.JoinHostPort(host, strconv.Itoa(fpcService.Port())),
		})
	}

	return services
}
