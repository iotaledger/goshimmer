package portcheck

import (
	"net"
	"sync"

	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/netutil"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Port Check", node.Enabled, run)

var log *logger.Logger

func run(ctx *node.Plugin) {
	log = logger.NewLogger("Tangle")

	lPeer := local.GetInstance()

	// use the port of the peering service
	peeringAddr := lPeer.Services().Get(service.PeeringKey)
	_, peeringPort, err := net.SplitHostPort(peeringAddr.String())
	if err != nil {
		panic(err)
	}

	// resolve the bind address
	address := net.JoinHostPort(config.Node.GetString(local.CFG_BIND), peeringPort)
	localAddr, err := net.ResolveUDPAddr(peeringAddr.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CFG_BIND, err)
	}

	// check that discovery is working and the port is open
	log.Info("Testing service ...")
	checkAutopeeringConnection(localAddr, &lPeer.Peer)
	log.Info("Testing service ... done")

	//check that the server is working and the port is open
	log.Info("Testing service ...")
	checkGossipConnection(gossip.Srv, &lPeer.Peer)
	log.Info("Testing service ... done")
}

func checkAutopeeringConnection(localAddr *net.UDPAddr, self *peer.Peer) {
	peering := self.Services().Get(service.PeeringKey)
	remoteAddr, err := net.ResolveUDPAddr(peering.Network(), peering.String())
	if err != nil {
		panic(err)
	}

	// do not check the address as a NAT may change them for local connections
	err = netutil.CheckUDP(localAddr, remoteAddr, false, true)
	if err != nil {
		log.Errorf("Error testing service: %s", err)
		log.Panicf("Please check that %s is publicly reachable at %s/%s",
			banner.AppName, peering.String(), peering.Network())
	}
}

func checkGossipConnection(srv *server.TCP, self *peer.Peer) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := srv.AcceptPeer(self)
		if err != nil {
			return
		}
		_ = conn.Close()
	}()
	conn, err := srv.DialPeer(self)
	if err != nil {
		log.Errorf("Error testing: %s", err)
		addr := self.Services().Get(service.GossipKey)
		log.Panicf("Please check that %s is publicly reachable at %s/%s",
			banner.AppName, addr.String(), addr.Network())
	}
	_ = conn.Close()
	wg.Wait()
}
