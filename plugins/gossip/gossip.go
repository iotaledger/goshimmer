package gossip

import (
	"fmt"
	"net"
	"strconv"

	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var (
	log *logger.Logger
	mgr *gp.Manager
	srv *server.TCP
)

func configureGossip() {
	lPeer := local.GetInstance()

	// announce the gossip service
	gossipPort := config.Node.GetInt(GOSSIP_PORT)
	if !netutil.IsValidPort(gossipPort) {
		log.Fatalf("Invalid port number (%s): %d", GOSSIP_PORT, gossipPort)
	}

	if err := lPeer.UpdateService(service.GossipKey, "tcp", gossipPort); err != nil {
		log.Fatalf("could not update services: %s", err)
	}
	mgr = gp.NewManager(lPeer, getTransaction, log)
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + name + " ... done")

	lPeer := local.GetInstance()

	// use the port of the gossip service
	gossipEndpoint := lPeer.Services().Get(service.GossipKey)

	// resolve the bind address
	address := net.JoinHostPort(config.Node.GetString(local.CFG_BIND), strconv.Itoa(gossipEndpoint.Port()))
	localAddr, err := net.ResolveTCPAddr(gossipEndpoint.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CFG_BIND, err)
	}

	listener, err := net.ListenTCP(gossipEndpoint.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv = server.ServeTCP(lPeer, listener, log)
	defer srv.Close()

	mgr.Start(srv)
	defer mgr.Close()

	log.Infof("%s started: Address=%s/%s", name, localAddr.String(), localAddr.Network())

	<-shutdownSignal
	log.Info("Stopping " + name + " ...")
}

func getTransaction(transactionId message.Id) (bytes []byte, err error) {
	log.Debugw("get tx from db", "id", transactionId.String())

	if !tangle.Instance.GetTransaction(transactionId).Consume(func(transaction *message.Transaction) {
		bytes = transaction.Bytes()
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
