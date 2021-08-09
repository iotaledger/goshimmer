package gossip

import (
	"net"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/netutil"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// ErrMessageNotFound is returned when a message could not be found in the Tangle.
var ErrMessageNotFound = errors.New("message not found")

var (
	mgr     *gossip.Manager
	mgrOnce sync.Once
)

// Manager returns the manager instance of the gossip plugin.
func Manager() *gossip.Manager {
	mgrOnce.Do(createManager)
	return mgr
}

func createManager() {
	// announce the gossip service
	gossipPort := Parameters.Port
	if !netutil.IsValidPort(gossipPort) {
		Plugin().LogFatalf("Invalid port number: %d", gossipPort)
	}

	lPeer := local.GetInstance()
	if err := lPeer.UpdateService(service.GossipKey, "tcp", gossipPort); err != nil {
		Plugin().LogFatalf("could not update services: %s", err)
	}
	mgr = gossip.NewManager(lPeer, loadMessage, Plugin().Logger())
}

func start(shutdownSignal <-chan struct{}) {
	defer Plugin().LogInfo("Stopping " + PluginName + " ... done")

	lPeer := local.GetInstance()

	// use the port of the gossip service
	gossipEndpoint := lPeer.Services().Get(service.GossipKey)

	// resolve the bind address
	address := net.JoinHostPort(config.Node().String(local.ParametersNetwork.BindAddress), strconv.Itoa(gossipEndpoint.Port()))
	localAddr, err := net.ResolveTCPAddr(gossipEndpoint.Network(), address)
	if err != nil {
		Plugin().LogFatalf("Error resolving: %v", err)
	}

	listener, err := net.ListenTCP(gossipEndpoint.Network(), localAddr)
	if err != nil {
		Plugin().LogFatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(lPeer, listener, Plugin().Logger())
	defer srv.Close()

	mgr.Start(srv)
	defer mgr.Stop()

	Plugin().LogInfof("%s started: bind-address=%s", PluginName, localAddr.String())

	<-shutdownSignal
	Plugin().LogInfo("Stopping " + PluginName + " ...")
}

// loads the given message from the message layer and returns it or an error if not found.
func loadMessage(msgID tangle.MessageID) ([]byte, error) {
	cachedMessage := messagelayer.Tangle().Storage.Message(msgID)
	defer cachedMessage.Release()
	if !cachedMessage.Exists() {
		return nil, ErrMessageNotFound
	}
	msg := cachedMessage.Unwrap()
	return msg.Bytes(), nil
}
