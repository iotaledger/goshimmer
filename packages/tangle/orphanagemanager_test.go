package tangle

import (
	"container/heap"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestOrphanageManager_removeElementFromHeap(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	orphanManager := tangle.OrphanageManager
	now := time.Now()
	msgIDs := make([]MessageID, 0)
	for i := 0; i < 20; i++ {
		msgIDs = append(msgIDs, randomMessageID())
		heap.Push(&orphanManager.unconfirmedBlocks, &QueueElement{Key: now.Add(time.Duration(i) * time.Second), Value: msgIDs[len(msgIDs)-1]})
	}

	orphanManager.removeElementFromHeap(msgIDs[10])
	assert.Len(t, orphanManager.unconfirmedBlocks, 19)
	orphanManager.removeElementFromHeap(msgIDs[10])
	assert.Len(t, orphanManager.unconfirmedBlocks, 19)
}

func TestOrphanageManager_orphanBeforeTSC(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	orphanManager := tangle.OrphanageManager
	orphanedBlocks := atomic.NewInt32(0)
	orphanManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
		orphanedBlocks.Inc()
	}))
	now := time.Now()
	msgIDs := make([]MessageID, 0)
	for i := 0; i < 20; i++ {
		msgIDs = append(msgIDs, randomMessageID())
		heap.Push(&orphanManager.unconfirmedBlocks, &QueueElement{Key: now.Add(time.Duration(i) * time.Second), Value: msgIDs[len(msgIDs)-1]})
	}

	orphanManager.orphanBeforeTSC(now.Add(time.Duration(10) * time.Second))
	assert.Len(t, orphanManager.unconfirmedBlocks, 10)

	assert.Equal(t, int32(10), orphanedBlocks.Load())
}

func TestOrphanageManager_OrphanBlock(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	orphanManager := tangle.OrphanageManager
	orphanedBlocks := atomic.NewInt32(0)
	orphanManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
		orphanedBlocks.Inc()
	}))
	confirmationOracle := &MockConfirmationOracleTipManagerTest{
		confirmedMessageIDs:    NewMessageIDs(),
		confirmedMarkers:       markers.NewMarkers(),
		MockConfirmationOracle: MockConfirmationOracle{},
	}
	tangle.ConfirmationOracle = confirmationOracle

	testFramework := NewMessageTestFramework(
		tangle,
	)

	tangle.Setup()

	createTestTangleOrphanage(testFramework)
	confirmedMessageIDsString := []string{"Marker-0/1", "0/1-preTSC_0", "0/1-preTSC_1", "0/1-preTSC_2", "0/1-preTSC_3", "0/1-preTSC_4", "0/1-preTSC_5", "0/1-preTSC_6", "0/1-preTSC_7", "0/1-preTSC_8", "0/1-preTSC_9", "0/1-postTSC_0"}
	confirmedMessageIDs := prepareConfirmedMessageIDs(testFramework, confirmedMessageIDsString)

	confirmationOracle.Lock()
	confirmationOracle.confirmedMessageIDs = confirmedMessageIDs
	confirmationOracle.Unlock()

	event.Loop.WaitUntilAllTasksProcessed()

	assert.Equal(t, 27, orphanManager.unconfirmedBlocks.Len())
	assert.Equal(t, 25, len(orphanManager.strongChildCounters))

	orphanManager.OrphanBlock(testFramework.Message("0/1-preTSCSeq1_0").ID(), errors.New("message orphaned"))
	event.Loop.WaitUntilAllTasksProcessed()

	assert.Equal(t, int32(15), orphanedBlocks.Load())
	assert.Equal(t, 12, orphanManager.unconfirmedBlocks.Len())
	assert.Equal(t, 11, len(orphanManager.strongChildCounters))
}

func TestOrphanageManager_AcceptanceTimeUpdatedEvent(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	orphanManager := tangle.OrphanageManager
	orphanedBlocks := atomic.NewInt32(0)
	orphanManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
		orphanedBlocks.Inc()
	}))

	confirmationOracle := &MockConfirmationOracleTipManagerTest{
		confirmedMessageIDs:    NewMessageIDs(),
		confirmedMarkers:       markers.NewMarkers(),
		MockConfirmationOracle: MockConfirmationOracle{},
	}
	tangle.ConfirmationOracle = confirmationOracle

	testFramework := NewMessageTestFramework(
		tangle,
	)

	tangle.Setup()

	createTestTangleOrphanage(testFramework)
	confirmedMessageIDsString := []string{"Marker-0/1", "0/1-preTSC_0", "0/1-preTSC_1", "0/1-preTSC_2", "0/1-preTSC_3", "0/1-preTSC_4", "0/1-preTSC_5", "0/1-preTSC_6", "0/1-preTSC_7", "0/1-preTSC_8", "0/1-preTSC_9", "0/1-postTSC_0"}
	confirmedMessageIDs := prepareConfirmedMessageIDs(testFramework, confirmedMessageIDsString)

	confirmationOracle.Lock()
	confirmationOracle.confirmedMessageIDs = confirmedMessageIDs
	confirmationOracle.Unlock()
	assert.Equal(t, 27, orphanManager.unconfirmedBlocks.Len())
	assert.Equal(t, 25, len(orphanManager.strongChildCounters))

	for _, blockID := range confirmedMessageIDsString {
		confirmationOracle.Events().MessageConfirmed.Trigger(&MessageConfirmedEvent{Message: testFramework.Message(blockID)})
	}
	event.Loop.WaitUntilAllTasksProcessed()

	assert.Equal(t, int32(15), orphanedBlocks.Load())
	assert.Equal(t, 0, orphanManager.unconfirmedBlocks.Len())
	assert.Equal(t, 0, len(orphanManager.strongChildCounters))
}

func createTestTangleOrphanage(testFramework *MessageTestFramework) {
	var lastMsgAlias string
	// SEQUENCE 0
	{
		testFramework.CreateMessage("Marker-0/1", WithStrongParents("Genesis"), WithIssuingTime(time.Now().Add(-6*time.Minute)))
		testFramework.IssueMessages("Marker-0/1").WaitUntilAllTasksProcessed()
		lastMsgAlias = issueMessages(testFramework, "0/1-preTSC", 10, []string{"Marker-0/1"}, time.Minute*6)
		lastMsgAlias = issueMessages(testFramework, "0/1-postTSC", 1, []string{lastMsgAlias}, 0)

	}

	// SEQUENCE 1
	{ //nolint:dupl
		lastMsgAlias = issueMessages(testFramework, "0/1-preTSCSeq1", 10, []string{"Marker-0/1"}, time.Minute*6)
		lastMsgAlias = issueMessages(testFramework, "0/1-postTSCSeq1", 5, []string{lastMsgAlias}, time.Minute)
	}
}
