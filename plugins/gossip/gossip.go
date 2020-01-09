package gossip

import (
	"fmt"
	"net"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/errors"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
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

	mgr.Start(srv)
	defer mgr.Close()

	log.Infof("Gossip started: address=%v", mgr.LocalAddr())

	<-shutdownSignal
	log.Info("Stopping Gossip ...")
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
