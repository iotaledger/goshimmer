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
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

// PluginName is the name of the web API autopeering endpoint plugin.
const PluginName = "WebAPI autopeering Endpoint"

var (
	// plugin is the plugin instance of the web API autopeering endpoint plugin.
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In

	Server    *echo.Echo
	Selection *selection.Protocol
	Discover  *discover.Protocol
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		Plugin.LogError(err)
	}
	deps.Server.GET("autopeering/neighbors", getNeighbors)
}

// getNeighbors returns the chosen and accepted neighbors of the node
func getNeighbors(c echo.Context) error {
	var chosen []jsonmodels.Neighbor
	var accepted []jsonmodels.Neighbor
	var knownPeers []jsonmodels.Neighbor

	if c.QueryParam("known") == "1" {
		for _, p := range deps.Discover.GetVerifiedPeers() {
			knownPeers = append(knownPeers, createNeighborFromPeer(p))
		}
	}

	for _, p := range deps.Selection.GetOutgoingNeighbors() {
		chosen = append(chosen, createNeighborFromPeer(p))
	}
	for _, p := range deps.Selection.GetIncomingNeighbors() {
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
