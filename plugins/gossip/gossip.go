package gossip

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var (
	log *logger.Logger
	mgr *gp.Manager
)

func configureGossip() {
	lPeer := local.GetInstance()

	peeringAddr := lPeer.Services().Get(service.PeeringKey)
	external, _, err := net.SplitHostPort(peeringAddr.String())
	if err != nil {
		panic(err)
	}

	// announce the gossip service
	gossipPort := strconv.Itoa(config.NodeConfig.GetInt(GOSSIP_PORT))
	err = lPeer.UpdateService(service.GossipKey, "tcp", net.JoinHostPort(external, gossipPort))
	if err != nil {
		log.Fatalf("could not update services: %s", err)
	}

	mgr = gp.NewManager(lPeer, getTransaction, log)
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + name + " ... done")

	lPeer := local.GetInstance()
	// use the port of the gossip service
	gossipAddr := lPeer.Services().Get(service.GossipKey)
	_, gossipPort, err := net.SplitHostPort(gossipAddr.String())
	if err != nil {
		panic(err)
	}
	// resolve the bind address
	address := net.JoinHostPort(config.NodeConfig.GetString(local.CFG_BIND), gossipPort)
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

	//check that the server is working and the port is open
	log.Info("Testing service ...")
	checkConnection(srv, &lPeer.Peer)
	log.Info("Testing service ... done")

	mgr.Start(srv)
	defer mgr.Close()

	log.Infof("%s started: Address=%s/%s", name, gossipAddr.String(), gossipAddr.Network())

	<-shutdownSignal
	log.Info("Stopping " + name + " ...")
}

func checkConnection(srv *server.TCP, self *peer.Peer) {
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

func getTransaction(transactionId transaction.Id) (bytes []byte, err error) {
	log.Debugw("get tx from db", "id", transactionId.String())

	if !tangle.Instance.GetTransaction(transactionId).Consume(func(transaction *transaction.Transaction) {
		bytes = transaction.GetBytes()
	}) {
		err = fmt.Errorf("transaction not found: hash=%s", transactionId)
	}

	return
}

func GetAllNeighbors() []*gp.Neighbor {
	if mgr == nil {
		return nil
	}
	return mgr.GetAllNeighbors()
}
