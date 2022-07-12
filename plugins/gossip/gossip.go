package gossip

import (
	"context"
	"fmt"
	"net"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto"
	"github.com/libp2p/go-libp2p"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/libp2putil"
	"github.com/iotaledger/goshimmer/packages/ratelimiter"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// ErrBlockNotFound is returned when a block could not be found in the Tangle.
var ErrBlockNotFound = errors.New("block not found")

var localAddr *net.TCPAddr

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

	// loads the given block from the block layer and returns it or an error if not found.
	loadBlock := func(blkID tangle.BlockID) ([]byte, error) {
		cachedBlock := t.Storage.Block(blkID)
		defer cachedBlock.Release()
		if !cachedBlock.Exists() {
			if crypto.Randomness.Float64() < Parameters.MissingBlockRequestRelayProbability {
				t.Solidifier.RetrieveMissingBlock(blkID)
			}

			return nil, ErrBlockNotFound
		}
		blk, _ := cachedBlock.Unwrap()
		return blk.Bytes()
	}
	libp2pIdentity, err := libp2putil.GetLibp2pIdentity(lPeer)
	if err != nil {
		Plugin.LogFatalf("Could not build libp2p identity from local peer: %s", err)
	}
	libp2pHost, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", localAddr.IP, localAddr.Port)),
		libp2pIdentity,
		libp2p.NATPortMap(),
	)
	if err != nil {
		Plugin.LogFatalf("Couldn't create libp2p host: %s", err)
	}
	var opts []gossip.ManagerOption
	if Parameters.BlocksRateLimit != (blocksLimitParameters{}) {
		Plugin.Logger().Infof("Initializing blocks rate limiter with the following parameters: %+v",
			Parameters.BlocksRateLimit)
		mrl, mrlErr := ratelimiter.NewPeerRateLimiter(
			Parameters.BlocksRateLimit.Interval, Parameters.BlocksRateLimit.Limit,
			Plugin.Logger().With("rateLimiter", "blocksRateLimiter"),
		)
		if mrlErr != nil {
			Plugin.LogFatalf("Failed to initialize blocks rate limiter: %+v", mrlErr)
		}
		opts = append(opts, gossip.WithBlocksRateLimiter(mrl))
	}
	if Parameters.BlockRequestsRateLimit != (blockRequestsLimitParameters{}) {
		Plugin.Logger().Infof("Initializing block requests rate limiter with the following parameters: %+v",
			Parameters.BlockRequestsRateLimit)
		mrrl, mrrlErr := ratelimiter.NewPeerRateLimiter(
			Parameters.BlockRequestsRateLimit.Interval, Parameters.BlockRequestsRateLimit.Limit,
			Plugin.Logger().With("rateLimiter", "blockRequestsRateLimiter"),
		)
		if mrrlErr != nil {
			Plugin.LogFatalf("Failed to initialize block requests rate limiter: %+v", mrrlErr)
		}
		opts = append(opts, gossip.WithBlockRequestsRateLimiter(mrrl))
	}
	mgr := gossip.NewManager(libp2pHost, lPeer, loadBlock, Plugin.Logger(), opts...)
	return mgr
}

func start(ctx context.Context) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")
	defer func() {
		if mrl := deps.GossipMgr.BlocksRateLimiter(); mrl != nil {
			mrl.Close()
		}
	}()
	defer func() {
		if mrrl := deps.GossipMgr.BlockRequestsRateLimiter(); mrrl != nil {
			mrrl.Close()
		}
	}()
	defer deps.GossipMgr.Stop()
	defer func() {
		if err := deps.GossipMgr.Libp2pHost.Close(); err != nil {
			Plugin.LogWarn("Failed to close libp2p host: %+v", err)
		}
	}()

	Plugin.LogInfof("%s started: bind-address=%s", PluginName, localAddr.String())

	<-ctx.Done()
	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
