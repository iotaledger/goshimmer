package gossip

import (
	"fmt"
	"net"
	"strconv"

	"github.com/iotaledger/autopeering-sim/logger"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/goshimmer/packages/errors"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/typeutils"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/parameter"
)

var (
	mgr *gp.Manager
)

const defaultZLC = `{
	"level": "info",
	"development": false,
	"outputPaths": ["./gossip.log"],
	"errorOutputPaths": ["stderr"],
	"encoding": "console",
	"encoderConfig": {
	  "timeKey": "ts",
	  "levelKey": "level",
	  "nameKey": "logger",
	  "callerKey": "caller",
	  "messageKey": "msg",
	  "stacktraceKey": "stacktrace",
	  "lineEnding": "",
	  "levelEncoder": "",
	  "timeEncoder": "iso8601",
	  "durationEncoder": "",
	  "callerEncoder": ""
	}
  }`

var zLogger = logger.NewLogger(defaultZLC, logLevel)

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

	mgr = gp.NewManager(lPeer, getTransaction, zLogger)
}

func start() {
	defer log.Info("Stopping Gossip ... done")

	srv, err := server.ListenTCP(local.GetInstance(), zLogger)
	if err != nil {
		log.Fatalf("ListenTCP: %v", err)
	}
	defer srv.Close()

	mgr.Start(srv)
	defer mgr.Close()

	log.Infof("Gossip started: address=%v", mgr.LocalAddr())

	<-daemon.ShutdownSignal
	log.Info("Stopping Gossip ...")
}

func getTransaction(hash []byte) ([]byte, error) {
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
