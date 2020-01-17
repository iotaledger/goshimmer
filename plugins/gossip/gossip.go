package gossip

import (
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/errors"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	log *logger.Logger
	mgr *gp.Manager
)

func configureGossip() {
	lPeer := local.GetInstance()

	port := strconv.Itoa(parameter.NodeConfig.GetInt(GOSSIP_PORT))

	host, _, err := net.SplitHostPort(lPeer.Address())
	if err != nil {
		log.Fatalf("invalid peering address: %v", err)
	}
	err = lPeer.UpdateService(service.GossipKey, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		log.Fatalf("could not update services: %v", err)
	}

	mgr = gp.NewManager(lPeer, loadTransaction, log)
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping Gossip ... done")

	srv, err := server.ListenTCP(local.GetInstance(), log)
	if err != nil {
		log.Fatalf("ListenTCP: %v", err)
	}
	defer srv.Close()

	//check that the server is working and the port is open
	log.Info("Testing server ...")
	checkConnection(srv, &local.GetInstance().Peer)
	log.Info("Testing server ... done")

	mgr.Start(srv)
	defer mgr.Close()

	log.Infof("Gossip started: address=%s/%s", mgr.LocalAddr().String(), mgr.LocalAddr().Network())

	<-shutdownSignal
	log.Info("Stopping Gossip ...")
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
	runtime.Gosched()
	conn, err := srv.DialPeer(self)
	if err != nil {
		if err == server.ErrTimeout {
			log.Panicf("Please check that %s is publicly reachable at %s/%s",
				cli.AppName, srv.LocalAddr().String(), srv.LocalAddr().Network())
		}
		log.Panicf("Error: %s", err)
	}
	_ = conn.Close()
	wg.Wait()
}

func loadTransaction(hash []byte) ([]byte, error) {
	log.Infof("Retrieving tx: hash=%s", hash)

	tx, err := tangle.GetTransaction(typeutils.BytesToString(hash))
	if err != nil {
		return nil, errors.Wrap(err, "could not get transaction")
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found: hash=%s", hash)
	}
	return tx.GetBytes(), nil
}

func requestTransaction(hash trinary.Hash) {
	if contains, _ := tangle.ContainsTransaction(hash); contains {
		// Do not request tx that we already know
		return
	}
	mgr.RequestTransaction(typeutils.StringToBytes(hash))
}
