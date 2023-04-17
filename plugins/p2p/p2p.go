package p2p

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p"

	"github.com/iotaledger/goshimmer/packages/core/libp2putil"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

var localAddr *net.TCPAddr

func createManager(lPeer *peer.Local) *p2p.Manager {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveTCPAddr("tcp", Parameters.BindAddress)
	if err != nil {
		Plugin.LogFatalfAndExitf("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the service
	if serviceErr := lPeer.UpdateService(service.P2PKey, localAddr.Network(), localAddr.Port); serviceErr != nil {
		Plugin.LogFatalfAndExitf("could not update services: %s", serviceErr)
	}

	libp2pIdentity, err := libp2putil.GetLibp2pIdentity(lPeer)
	if err != nil {
		Plugin.LogFatalfAndExitf("Could not build libp2p identity from local peer: %s", err)
	}
	libp2pHost, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", localAddr.IP, localAddr.Port)),
		libp2pIdentity,
		libp2p.NATPortMap(),
	)
	if err != nil {
		Plugin.LogFatalfAndExitf("Couldn't create libp2p host: %s", err)
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
