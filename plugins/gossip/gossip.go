package gossip

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"
)

var (
	log     *logger.Logger
	mgr     *gossip.Manager
	mgrOnce sync.Once
)

// Manager returns the manager instance of the gossip plugin.
func Manager() *gossip.Manager {
	mgrOnce.Do(createManager)
	return mgr
}

func createManager() {
	log = logger.NewLogger(PluginName)
	lPeer := local.GetInstance()

	// announce the gossip service
	gossipPort := config.Node.GetInt(CfgGossipPort)
	if !netutil.IsValidPort(gossipPort) {
		log.Fatalf("Invalid port number (%s): %d", CfgGossipPort, gossipPort)
	}

	if err := lPeer.UpdateService(service.GossipKey, "tcp", gossipPort); err != nil {
		log.Fatalf("could not update services: %s", err)
	}
	mgr = gossip.NewManager(lPeer, loadMessage, log)
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + PluginName + " ... done")

	lPeer := local.GetInstance()

	// use the port of the gossip service
	gossipEndpoint := lPeer.Services().Get(service.GossipKey)

	// resolve the bind address
	address := net.JoinHostPort(config.Node.GetString(local.CfgBind), strconv.Itoa(gossipEndpoint.Port()))
	localAddr, err := net.ResolveTCPAddr(gossipEndpoint.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CfgBind, err)
	}

	listener, err := net.ListenTCP(gossipEndpoint.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(lPeer, listener, log)
	defer srv.Close()

	mgr.Start(srv)
	defer mgr.Close()

	log.Infof("%s started: Address=%s/%s", PluginName, localAddr.String(), localAddr.Network())

	<-shutdownSignal
	log.Info("Stopping " + PluginName + " ...")
}

// loads the given message from the message layer or an error if not found.
func loadMessage(messageID message.Id) (bytes []byte, err error) {
	log.Debugw("load message from db", "id", messageID.String())
	if !messagelayer.Tangle.Message(messageID).Consume(func(message *message.Message) {
		bytes = message.Bytes()
	}) {
		err = fmt.Errorf("message not found: hash=%s", messageID)
	}
	return
}
