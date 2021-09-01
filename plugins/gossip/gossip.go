package gossip

import (
	"net"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer/service"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// ErrMessageNotFound is returned when a message could not be found in the Tangle.
var ErrMessageNotFound = errors.New("message not found")

var (
	mgr     *gossip.Manager
	mgrOnce sync.Once

	localAddr *net.TCPAddr
)

// Manager returns the manager instance of the gossip plugin.
func Manager() *gossip.Manager {
	mgrOnce.Do(createManager)
	return mgr
}

func createManager() {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveTCPAddr("tcp", Parameters.BindAddress)
	if err != nil {
		Plugin().LogFatalf("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	lPeer := local.GetInstance()

	// announce the gossip service
	if err := lPeer.UpdateService(service.GossipKey, localAddr.Network(), localAddr.Port); err != nil {
		Plugin().LogFatalf("could not update services: %s", err)
	}
	mgr = gossip.NewManager(lPeer, loadMessage, Plugin().Logger())
}

func start(shutdownSignal <-chan struct{}) {
	defer Plugin().LogInfo("Stopping " + PluginName + " ... done")

	// assure that the manager is initialized
	mgr := Manager()

	listener, err := net.ListenTCP(localAddr.Network(), localAddr)
	if err != nil {
		Plugin().LogFatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(local.GetInstance(), listener, Plugin().Logger())
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
