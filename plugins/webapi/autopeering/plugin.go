package autopeering

import (
	"net"
	"net/http"
	"strconv"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

// PluginName is the name of the web API autopeering endpoint plugin.
const PluginName = "WebAPIAutopeeringEndpoint"

var (
	// Plugin is the plugin instance of the web API autopeering endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server    *echo.Echo
	Selection *selection.Protocol `optional:"true"`
	Discover  *discover.Protocol  `optional:"true"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("autopeering/neighbors", getNeighbors)
}

// getNeighbors returns the chosen and accepted neighbors of the node
func getNeighbors(c echo.Context) error {
	var chosen []jsonmodels.Neighbor
	var accepted []jsonmodels.Neighbor
	var knownPeers []jsonmodels.Neighbor

	if c.QueryParam("known") == "1" {
		if deps.Discover != nil {
			for _, p := range deps.Discover.GetVerifiedPeers() {
				knownPeers = append(knownPeers, createNeighborFromPeer(p))
			}
		}
	}

	if deps.Selection != nil {
		for _, p := range deps.Selection.GetOutgoingNeighbors() {
			chosen = append(chosen, createNeighborFromPeer(p))
		}
		for _, p := range deps.Selection.GetIncomingNeighbors() {
			accepted = append(accepted, createNeighborFromPeer(p))
		}
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

	return services
}
