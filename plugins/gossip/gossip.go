package gossip

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/app/ratelimiter"
	"github.com/iotaledger/goshimmer/packages/node/gossip"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
)

// ErrBlockNotFound is returned when a block could not be found in the Tangle.
var ErrBlockNotFound = errors.New("block not found")

func createManager(p2pManager *p2p.Manager, t *tangleold.Tangle) *gossip.Manager {
	// loads the given block from the block layer and returns it or an error if not found.
	loadBlock := func(blkID tangleold.BlockID) ([]byte, error) {
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
	var opts []gossip.ManagerOption
	if Parameters.BlocksRateLimit != (blocksLimitParameters{}) {
		Plugin.Logger().Infof("Initializing blocks rate limiter with the following parameters: %+v",
			Parameters.BlocksRateLimit)
		mrl, mrlErr := ratelimiter.NewPeerRateLimiter(
			Parameters.BlocksRateLimit.Interval, Parameters.BlocksRateLimit.Limit,
			Plugin.Logger().With("rateLimiter", "blocksRateLimiter"),
		)
		if mrlErr != nil {
			Plugin.LogFatalfAndExit("Failed to initialize blocks rate limiter: %+v", mrlErr)
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
			Plugin.LogFatalfAndExit("Failed to initialize block requests rate limiter: %+v", mrrlErr)
		}
		opts = append(opts, gossip.WithBlockRequestsRateLimiter(mrrl))
	}
	mgr := gossip.NewManager(p2pManager, loadBlock, Plugin.Logger(), opts...)
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

	<-ctx.Done()
	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
