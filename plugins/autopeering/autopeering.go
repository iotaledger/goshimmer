package autopeering

import (
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	// Conn contains the network connection.
	Conn *NetConnMetric
)

var (
	// the peer selection protocol
	peerSel     *selection.Protocol
	peerSelOnce sync.Once

	// block until the peering server has been started
	srvBarrier = struct {
		once sync.Once
		c    chan *server.Server
	}{c: make(chan *server.Server, 1)}
)

// Selection returns the neighbor selection instance.
func Selection() *selection.Protocol {
	peerSelOnce.Do(createPeerSel)
	return peerSel
}

// BindAddress returns the string form of the autopeering bind address.
func BindAddress() string {
	peering := local.GetInstance().Services().Get(service.PeeringKey)
	host := config.Node().String(local.CfgBind)
	port := strconv.Itoa(peering.Port())
	return net.JoinHostPort(host, port)
}

// StartSelection starts the neighbor selection process.
// It blocks until the peer discovery has been started. Multiple calls of StartSelection are ignored.
func StartSelection() {
	srvBarrier.once.Do(func() {
		srv := <-srvBarrier.c
		close(srvBarrier.c)

		Selection().Start(srv)
	})
}

func createPeerSel() {
	// assure that the logger is available
	log := logger.NewLogger(PluginName).Named("sel")

	peerSel = selection.New(local.GetInstance(), discovery.Discovery(),
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

	lPeer := local.GetInstance()
	peering := lPeer.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), BindAddress())
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CfgBind, err)
	}

	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	Conn = &NetConnMetric{UDPConn: conn}

	// start a server doing peerDisc and peering
	srv := server.Serve(lPeer, Conn, log.Named("srv"), discovery.Discovery(), Selection())
	defer srv.Close()

	// start the peer discovery on that connection
	discovery.Discovery().Start(srv)
	srvBarrier.c <- srv

	log.Infof("%s started: ID=%s Address=%s/%s", PluginName, lPeer.ID(), localAddr.String(), localAddr.Network())

	<-shutdownSignal

	log.Infof("Stopping %s ...", PluginName)

	discovery.Discovery().Close()
	Selection().Close()

	lPeer.Database().Close()
}

func evalMana(nodeIdentity *identity.Identity) uint64 {
	m, _, err := messagelayer.GetConsensusMana(nodeIdentity.ID())
	if err != nil {
		return 0
	}
	return uint64(m)
}
