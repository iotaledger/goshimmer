package inmemoryblockdag

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func NewTestBlockDAG(t *testing.T, workers *workerpool.Group, evictionState *eviction.State, slotTimeProvider *slot.TimeProvider, commitmentLoadFunc func(index slot.Index) (commitment *commitment.Commitment, err error), optsBlockDAG ...options.Option[BlockDAG]) *BlockDAG {
	require.NotNil(t, evictionState)
	return New(workers, evictionState, func() *slot.TimeProvider { return slotTimeProvider }, commitmentLoadFunc, optsBlockDAG...)
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, optsBlockDAG ...options.Option[BlockDAG]) *blockdag.TestFramework {
	storageInstance := blockdag.NewTestStorage(t, workers)
	b := NewTestBlockDAG(t, workers.CreateGroup("BlockDAG"), eviction.NewState(storageInstance), slot.NewTimeProvider(time.Now().Unix(), 10), blockdag.DefaultCommitmentFunc, optsBlockDAG...)

	return blockdag.NewTestFramework(t,
		workers.CreateGroup("BlockDAGTestFramework"),
		b,
		b.slotTimeProviderFunc,
	)
}
