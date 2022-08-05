package p2p

import (
	"context"
	"fmt"
	"net"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/libp2p/go-libp2p"

	"github.com/iotaledger/goshimmer/packages/node/libp2putil"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
)

var localAddr *net.TCPAddr

func createManager(lPeer *peer.Local) *p2p.Manager {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveTCPAddr("tcp", Parameters.BindAddress)
	if err != nil {
		Plugin.LogFatalfAndExit("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the service
	if err := lPeer.UpdateService(service.P2PKey, localAddr.Network(), localAddr.Port); err != nil {
		Plugin.LogFatalfAndExit("could not update services: %s", err)
	}

	libp2pIdentity, err := libp2putil.GetLibp2pIdentity(lPeer)
	if err != nil {
		Plugin.LogFatalfAndExit("Could not build libp2p identity from local peer: %s", err)
	}
	libp2pHost, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", localAddr.IP, localAddr.Port)),
		libp2pIdentity,
		libp2p.NATPortMap(),
	)
	if err != nil {
		Plugin.LogFatalfAndExit("Couldn't create libp2p host: %s", err)
	}

	return p2p.NewManager(libp2pHost, lPeer, Plugin.Logger())
}

func start(ctx context.Context) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")
	defer deps.P2PMgr.Stop()
	defer func() {
		if err := deps.P2PMgr.GetP2PHost().Close(); err != nil {
			Plugin.LogWarn("Failed to close libp2p host: %+v", err)
		}
	}()

	Plugin.LogInfof("%s started: bind-address=%s", PluginName, localAddr.String())

	<-ctx.Done()
	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
