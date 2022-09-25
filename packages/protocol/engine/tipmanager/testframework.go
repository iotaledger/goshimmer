package tipmanager

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	TipManager           *TipManager
	mockAcceptance       *acceptance.MockAcceptanceGadget
	scheduledBlocks      *shrinkingmap.ShrinkingMap[models.BlockID, *scheduler.Block]
	scheduledBlocksMutex sync.RWMutex

	test       *testing.T
	tipAdded   uint32
	tipRemoved uint32

	optsGenesisTime       time.Time
	optsClock             *clock.Clock
	optsTipManagerOptions []options.Option[TipManager]
	optsTangleOptions     []options.Option[tangle.Tangle]
	*tangle.TestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test:            test,
		mockAcceptance:  acceptance.NewMockAcceptanceGadget(),
		scheduledBlocks: shrinkingmap.New[models.BlockID, *scheduler.Block](),
		optsGenesisTime: time.Now().Add(-5 * time.Hour),
	}, opts, func(t *TestFramework) {
		t.TestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangleOptions(t.optsTangleOptions...),
		)
		if t.optsClock == nil {
			t.optsClock = clock.New()
		}
		t.optsClock.SetAcceptedTime(t.optsGenesisTime)

		if t.TipManager == nil {
			t.TipManager = New(t.TestFramework.Tangle, t.mockAcceptance, t.mockSchedulerBlock, t.optsClock.AcceptedTime, t.optsGenesisTime, t.optsTipManagerOptions...)
		}

	}, (*TestFramework).setupEvents, (*TestFramework).createGenesis)
}

func (t *TestFramework) setupEvents() {
	t.Tangle.Events.VirtualVoting.BlockTracked.Hook(event.NewClosure(func(block *virtualvoting.Block) {
		if debug.GetEnabled() {
			t.test.Logf("SIMULATING SCHEDULED: %s", block.ID())
		}

		t.scheduledBlocksMutex.Lock()
		scheduledBlock := scheduler.NewBlock(block, scheduler.WithScheduled(true))
		t.scheduledBlocks.Set(block.ID(), scheduledBlock)
		t.scheduledBlocksMutex.Unlock()

		t.TipManager.AddTip(scheduledBlock)
	}))

	t.TipManager.Events.TipAdded.Hook(event.NewClosure(func(block *scheduler.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TIP ADDED: %s", block.ID())
		}
		atomic.AddUint32(&(t.tipAdded), 1)
	}))

	t.TipManager.Events.TipRemoved.Hook(event.NewClosure(func(block *scheduler.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TIP REMOVED: %s", block.ID())
		}
		atomic.AddUint32(&(t.tipRemoved), 1)
	}))
}

func (t *TestFramework) createGenesis() {
	genesisMarker := markers.NewMarker(0, 0)
	structureDetails := markers.NewStructureDetails()
	structureDetails.SetPastMarkers(markers.NewMarkers(genesisMarker))
	structureDetails.SetIsPastMarker(true)
	structureDetails.SetPastMarkerGap(0)

	block := scheduler.NewBlock(
		virtualvoting.NewBlock(
			booker.NewBlock(
				blockdag.NewBlock(
					models.NewEmptyBlock(models.EmptyBlockID, models.WithIssuingTime(t.optsGenesisTime)),
					blockdag.WithSolid(true),
				),
				booker.WithBooked(true),
				booker.WithStructureDetails(structureDetails),
			),
		),
		scheduler.WithScheduled(true),
	)

	t.scheduledBlocks.Set(block.ID(), block)

	t.SetBlocksAccepted("Genesis")
	t.SetMarkersAccepted(genesisMarker)
}

func (t *TestFramework) mockSchedulerBlock(id models.BlockID) (block *scheduler.Block, exists bool) {
	t.scheduledBlocksMutex.RLock()
	defer t.scheduledBlocksMutex.RUnlock()

	return t.scheduledBlocks.Get(id)
}

func (t *TestFramework) IssueBlocksAndSetAccepted(aliases ...string) *blockdag.TestFramework {
	t.SetBlocksAccepted(aliases...)

	return t.IssueBlocks(aliases...)
}

func (t *TestFramework) SetBlocksAccepted(aliases ...string) {
	t.mockAcceptance.SetBlocksAccepted(t.BlockIDs(aliases...))
}

func (t *TestFramework) SetMarkersAccepted(m ...markers.Marker) {
	t.mockAcceptance.SetMarkersAccepted(m...)
}

func (t *TestFramework) SetAcceptedTime(acceptedTime time.Time) {
	t.optsClock.SetAcceptedTime(acceptedTime)
}

func (t *TestFramework) AssertIsPastConeTimestampCorrect(blockAlias string, expected bool) {
	block, exists := t.mockSchedulerBlock(t.Block(blockAlias).ID())
	if !exists {
		panic(fmt.Sprintf("block with %s not found", blockAlias))
	}
	actual := t.TipManager.isPastConeTimestampCorrect(block)
	assert.Equal(t.test, expected, actual, "isPastConeTimestampCorrect: %s should be %t but is %t", blockAlias, expected, actual)
}

func (t *TestFramework) AssertTipsAdded(count uint32) {
	assert.Equal(t.test, count, atomic.LoadUint32(&t.tipAdded), "expected %d tips to be added but got %d", count, atomic.LoadUint32(&t.tipAdded))
}

func (t *TestFramework) AssertTipsRemoved(count uint32) {
	assert.Equal(t.test, count, atomic.LoadUint32(&t.tipRemoved), "expected %d tips to be removed but got %d", count, atomic.LoadUint32(&t.tipRemoved))
}

func (t *TestFramework) AssertTips(actualTips scheduler.Blocks, expectedStateAliases ...string) {
	actualTipsIDs := models.NewBlockIDs()

	for it := actualTips.Iterator(); it.HasNext(); {
		actualTipsIDs.Add(it.Next().ID())
	}

	assert.Equal(t.test, len(expectedStateAliases), actualTips.Size(), "expected %d tips but got %d", len(expectedStateAliases), actualTips.Size())
	for _, expectedBlockAlias := range expectedStateAliases {
		t.VirtualVotingTestFramework.AssertBlock(expectedBlockAlias, func(block *virtualvoting.Block) {
			assert.True(t.test, actualTipsIDs.Contains(block.ID()), "block %s is not in the selected tips", block.ID())
		})
	}
}

func (t *TestFramework) AssertTipCount(expectedTipCount int) {
	assert.Equal(t.test, expectedTipCount, t.TipManager.TipCount(), "expected %d tip count but got %d", t.TipManager.TipCount(), expectedTipCount)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTipManagerOptions(opts ...options.Option[TipManager]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTipManagerOptions = opts
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangleOptions = opts
	}
}

func WithClock(c *clock.Clock) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsClock = c
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////