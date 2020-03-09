package portcheck

import (
	"net"
	"sync"

	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"

	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"
	"github.com/iotaledger/hive.go/node"
)

const (
	PLUGIN_NAME = "PortCheck"
)

var (
	PLUGIN = node.NewPlugin(PLUGIN_NAME, node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)
}

func run(ctx *node.Plugin) {
	if !node.IsSkipped(autopeering.PLUGIN) {
		log.Info("Testing autopeering service ...")
		checkAutopeeringConnection()
		log.Info("Testing autopeering service ... done")
	}

	if !node.IsSkipped(gossip.PLUGIN) {
		log.Info("Testing gossip service ...")
		checkGossipConnection()
		log.Info("Testing gossip service ... done")
	}
}

// check that discovery is working and the port is open
func checkAutopeeringConnection() {
	peering := local.GetInstance().Services().Get(service.PeeringKey)

	// use the port of the peering service
	_, peeringPort, err := net.SplitHostPort(peering.String())
	if err != nil {
		panic(err)
	}

	// resolve the bind address
	address := net.JoinHostPort(config.Node.GetString(local.CFG_BIND), peeringPort)
	localAddr, err := net.ResolveUDPAddr(peering.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CFG_BIND, err)
	}

	remoteAddr, err := net.ResolveUDPAddr(peering.Network(), peering.String())
	if err != nil {
		panic(err)
	}

	// do not check address and port as a NAT may change them for local connections
	err = netutil.CheckUDP(localAddr, remoteAddr, false, false)
	if err != nil {
		log.Errorf("Error testing autopeering service: %s", err)
		log.Panicf("Please check that %s is publicly reachable at %s/%s",
			banner.AppName, peering.String(), peering.Network())
	}
}

// check that the gossip server is working and the port is open
func checkGossipConnection() {
	// listen on TCP gossip port
	lPeer := local.GetInstance()
	// use the port of the gossip service
	gossipAddr := lPeer.Services().Get(service.GossipKey)
	_, gossipPort, err := net.SplitHostPort(gossipAddr.String())
	if err != nil {
		panic(err)
	}

	// resolve the bind address
	address := net.JoinHostPort(config.Node.GetString(local.CFG_BIND), gossipPort)
	localAddr, err := net.ResolveTCPAddr(gossipAddr.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CFG_BIND, err)
	}

	listener, err := net.ListenTCP(gossipAddr.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(lPeer, listener, log)
	defer srv.Close()

	// do the actual check
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, acceptErr := srv.AcceptPeer(&lPeer.Peer)
		if acceptErr != nil {
			return
		}
		_ = conn.Close()
	}()
	conn, err := srv.DialPeer(&lPeer.Peer)
	if err != nil {
		log.Errorf("Error testing: %s", err)
		log.Panicf("Please check that %s is publicly reachable at %s/%s",
			banner.AppName, gossipAddr.String(), gossipAddr.Network())
	}
	_ = conn.Close()
	wg.Wait()
}
