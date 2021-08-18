package autopeering

import (
	"net"
	"strconv"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// Conn contains the network connection.
var Conn *NetConnMetric

// BindAddress returns the string form of the autopeering bind address.
func BindAddress() string {
	peering := deps.Local.Services().Get(service.PeeringKey)
	port := strconv.Itoa(peering.Port())
	return net.JoinHostPort(local.ParametersNetwork.BindAddress, port)
}

func createPeerSel(localID *peer.Local, nbrDiscover *discover.Protocol) *selection.Protocol {
	// assure that the logger is available
	log := logger.NewLogger(PluginName).Named("sel")

	return selection.New(localID, nbrDiscover,
		selection.Logger(log),
		selection.NeighborValidator(selection.ValidatorFunc(isValidNeighbor)),
		selection.UseMana(Parameters.Mana),
		selection.ManaFunc(evalMana),
		selection.R(Parameters.R),
		selection.Ro(Parameters.Ro),
	)
}

// isValidNeighbor checks whether a peer is a valid neighbor.
func isValidNeighbor(p *peer.Peer) bool {
	// gossip must be supported
	gossipService := p.Services().Get(service.GossipKey)
	if gossipService == nil {
		return false
	}
	// gossip service must be valid
	if gossipService.Network() != "tcp" || gossipService.Port() < 0 || gossipService.Port() > 65535 {
		return false
	}
	return true
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + PluginName + " ... done")

	lPeer := deps.Local
	peering := lPeer.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), BindAddress())
	if err != nil {
		log.Fatalf("Error resolving: %v", err)
	}

	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	Conn = &NetConnMetric{UDPConn: conn}

	// start a server doing peerDisc and peering
	srv := server.Serve(lPeer, Conn, log.Named("srv"), deps.Discovery, deps.Selection)
	defer srv.Close()

	// start the peer discovery on that connection
	deps.Discovery.Start(srv)

	// start the neighbor selection process.
	deps.Selection.Start(srv)

	log.Infof("%s started: ID=%s Address=%s/%s", PluginName, lPeer.ID(), localAddr.String(), localAddr.Network())

	<-shutdownSignal

	log.Infof("Stopping %s ...", PluginName)
	deps.Selection.Close()

	deps.Discovery.Close()

	lPeer.Database().Close()
}

func evalMana(nodeIdentity *identity.Identity) uint64 {
	if !manaEnabled {
		return 0
	}
	m, _, err := messagelayer.GetConsensusMana(nodeIdentity.ID())
	if err != nil {
		return 0
	}
	return uint64(m)
}
