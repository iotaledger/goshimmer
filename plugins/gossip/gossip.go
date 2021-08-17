package gossip

import (
	"net"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/netutil"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
)

// ErrMessageNotFound is returned when a message could not be found in the Tangle.
var ErrMessageNotFound = errors.New("message not found")

func createManager(lPeer *peer.Local, t *tangle.Tangle) *gossip.Manager {
	// announce the gossip service
	gossipPort := Parameters.Port
	if !netutil.IsValidPort(gossipPort) {
		Plugin.LogFatalf("Invalid port number: %d", gossipPort)
	}

	if err := lPeer.UpdateService(service.GossipKey, "tcp", gossipPort); err != nil {
		Plugin.LogFatalf("could not update services: %s", err)
	}

	// loads the given message from the message layer and returns it or an error if not found.
	loadMessage := func(msgID tangle.MessageID) ([]byte, error) {
		cachedMessage := t.Storage.Message(msgID)
		defer cachedMessage.Release()
		if !cachedMessage.Exists() {
			return nil, ErrMessageNotFound
		}
		msg := cachedMessage.Unwrap()
		return msg.Bytes(), nil
	}

	return gossip.NewManager(lPeer, loadMessage, Plugin.Logger())
}

func start(shutdownSignal <-chan struct{}) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")

	lPeer := deps.Local

	// use the port of the gossip service
	gossipEndpoint := lPeer.Services().Get(service.GossipKey)

	// resolve the bind address
	address := net.JoinHostPort(deps.Node.String(local.ParametersNetwork.BindAddress), strconv.Itoa(gossipEndpoint.Port()))
	localAddr, err := net.ResolveTCPAddr(gossipEndpoint.Network(), address)
	if err != nil {
		Plugin.LogFatalf("Error resolving: %v", err)
	}

	listener, err := net.ListenTCP(gossipEndpoint.Network(), localAddr)
	if err != nil {
		Plugin.LogFatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(lPeer, listener, Plugin.Logger())
	defer srv.Close()

	deps.GossipMgr.Start(srv)
	defer deps.GossipMgr.Stop()

	Plugin.LogInfof("%s started: bind-address=%s", PluginName, localAddr.String())

	<-shutdownSignal
	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
