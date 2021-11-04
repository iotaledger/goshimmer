package gossip

import (
	"context"
	"net"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// ErrMessageNotFound is returned when a message could not be found in the Tangle.
var ErrMessageNotFound = errors.New("message not found")

var (
	localAddr *net.TCPAddr
)

func createManager(lPeer *peer.Local, t *tangle.Tangle) *gossip.Manager {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveTCPAddr("tcp", Parameters.BindAddress)
	if err != nil {
		Plugin.LogFatalf("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the gossip service
	if err := lPeer.UpdateService(service.GossipKey, localAddr.Network(), localAddr.Port); err != nil {
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

func start(ctx context.Context) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")

	lPeer := deps.Local

	listener, err := net.ListenTCP(localAddr.Network(), localAddr)
	if err != nil {
		Plugin.LogFatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(lPeer, listener, Plugin.Logger())
	defer srv.Close()

	deps.GossipMgr.Start(srv)
	defer deps.GossipMgr.Stop()

	Plugin.LogInfof("%s started: bind-address=%s", PluginName, localAddr.String())

	<-ctx.Done()
	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
