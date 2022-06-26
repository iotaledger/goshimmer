package notarization

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 1), testTangle)
	assert.NotNil(t, m)
}

func TestManager_pendingConflictsCounters(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 1), testTangle)
	m.pendingConflictsCounters[3] = 3
	assert.Equal(t, uint64(3), m.pendingConflictsCounters[3])
}

func TestManager_IsCommittable(t *testing.T) {
	m := testNotarizationManager()
	Index := epoch.Index(5)
	m.pendingConflictsCounters[Index] = 0
	// not old enough
	assert.False(t, m.isCommittable(Index))

	Index = epoch.Index(1)
	m.pendingConflictsCounters[Index] = 1
	// old enough but pbc > 0
	assert.False(t, m.isCommittable(Index))
	m.pendingConflictsCounters[Index] = 0
	// old enough and pbc > 0
	assert.True(t, m.isCommittable(Index))
}

func TestManager_GetLatestEC(t *testing.T) {
	m := testNotarizationManager()
	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 0; i <= 5; i++ {
		m.pendingConflictsCounters[epoch.Index(i)] = uint64(i)
		err := m.epochCommitmentFactory.insertTangleLeaf(epoch.Index(i), tangle.EmptyMessageID)
		require.NoError(t, err)
	}

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.Index(0), commitment.EI)

	m.pendingConflictsCounters[4] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, epoch.Index(0), commitment.EI)

	m.pendingConflictsCounters[2] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.Index(2), commitment.EI)
}

func TestManager_UpdateTangleTree(t *testing.T) {
	var epochInterval = 1 * time.Second

	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangle.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 20,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 25,
		}
	}
	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)

	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, epochInterval, []tangle.Option{tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false))}...)

	// Message1, issuing time epoch 1
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message1", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message2, issuing time epoch 2
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message3, issuing time epoch 3
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}
	// Message4, issuing time epoch 4
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message4", tangle.WithStrongParents("Message3", "Message2"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(2), msg.EI())
	}
	// Message5, issuing time epoch 5
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message5", tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(3), msg.EI())
		assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
			"Message1": true,
			"Message2": true,
			"Message3": true,
		})
	}
	eventHandlerMock.AssertExpectations(t)
}

func TestManager_UpdateStateMutationTree(t *testing.T) {
	var epochInterval = 1 * time.Second

	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangle.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, epochInterval, []tangle.Option{tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false))}...)

	// Message1, issuing time epoch 1
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message1", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message2, issuing time epoch 2
	{
		time.Sleep(epochInterval)
		fmt.Println("message 2")
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message3, issuing time epoch 3
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}
	// Message4, issuing time epoch 4
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message4", tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(2), msg.EI())
	}
	// Message5 TX1, issuing time epoch 5
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message5", tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(3), msg.EI())
	}
	// Message6 TX2, issuing time epoch 5
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message6", tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["E"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(3), msg.EI())
	}
	// Message7, issuing time epoch 6
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(4))
		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message7", tangle.WithStrongParents("Message6"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("C"), tangle.WithOutput("E", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(4), msg.EI())
	}
	// Message8, issuing time epoch 6
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(4))
		fmt.Println("message 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message8", tangle.WithStrongParents("Message7"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(4), msg.EI())
		assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
			"Message5": true,
			"Message6": true,
		})
	}

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_UpdateStateMutationTreeWithConflict(t *testing.T) {
	var epochInterval = 1 * time.Second

	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangle.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, epochInterval, []tangle.Option{tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false))}...)

	// Message1, issuing time epoch 1
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message1", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message2, issuing time epoch 1
	{
		fmt.Println("message 2")
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message3, issuing time epoch 1
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message4, issuing time epoch 1
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message4", tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message5 TX1, issuing time epoch 2
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message5", tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("B", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message6 TX2, issuing time epoch 2
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message6", tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["E"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message7, issuing time epoch 3
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))
		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message7", tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("E", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}
	// Message8, issuing time epoch 4
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))
		fmt.Println("message 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message8", tangle.WithStrongParents("Message7"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(2), msg.EI())
		assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
			"Message1": true,
			"Message2": true,
			"Message3": true,
			"Message4": true,
			"Message5": true,
			"Message7": true,
			"Message6": false,
		})
		assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
			"Message5": true,
			"Message7": true,
			"Message6": false,
		})
	}

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_TransactionInclusionUpdate(t *testing.T) {
	var epochInterval = 3 * time.Second

	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangle.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, epochInterval, []tangle.Option{tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false))}...)

	// Message1, issuing time epoch 1
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message1", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message2, issuing time epoch 1
	{
		fmt.Println("message 2")
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message3 TX1, issuing time epoch 1
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message4 TX2, issuing time epoch 1
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message4", tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())

		// pre-create message 8
		testFramework.CreateMessage("Message8", tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithInputs("C"), tangle.WithOutput("E", 500), tangle.WithECRecord(ecRecord))
	}
	// Message5, issuing time epoch 2
	{
		time.Sleep(epochInterval)
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message5", tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message6, issuing time epoch 1
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message6", tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithReattachment("Message8"), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message7, issuing time epoch 1
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message7", tangle.WithStrongParents("Message6"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message8, issuing time epoch 0, earlier attachment of Message6, with same tx
	{
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message9, issuing time epoch 2
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 9")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message9", tangle.WithStrongParents("Message8", "Message7"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message9").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message9")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message10, issuing time epoch 2
	{
		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
		fmt.Println("message 10")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message10", tangle.WithStrongParents("Message9"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message10").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message10")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Message3": true,
		"Message4": true,
		"Message8": true,
	})

	assertDiffStores(t, testFramework, notarizationMgr, map[epoch.Index][]string{
		epoch.Index(1): {"Message3", "Message4", "Message8"},
		epoch.Index(2): {},
	})

	// The transaction should be moved to the earlier epoch
	p, err := notarizationMgr.GetTransactionInclusionProof(testFramework.Transaction("Message6").ID())
	require.NoError(t, err)
	assert.Equal(t, epoch.Index(1), p.EI)

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_DiffUTXOs(t *testing.T) {
	var epochInterval = 1 * time.Second

	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangle.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, epochInterval, []tangle.Option{tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false))}...)

	// Message1, issuing time epoch 1
	{
		time.Sleep(epochInterval)
		fmt.Println("message 1")

		eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message1", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C1", 400), tangle.WithOutput("C1+", 100), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	// Message2, issuing time epoch 1
	{
		fmt.Println("message 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D2", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(1), []string{"A"}, []string{"C1", "C1+"})

	// Message3, issuing time epoch 1
	{
		time.Sleep(epochInterval)
		fmt.Println("message 3")

		//eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message3", tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	// Message4, issuing time epoch 1
	{
		fmt.Println("message 4")

		//eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message4", tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(1), []string{"A", "B"}, []string{"C1", "C1+", "D2"})

	/*
		// Message4, issuing time epoch 2
		{
			time.Sleep(epochInterval)
			fmt.Println("message 4")

			eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))

			ecRecord, _, err := testFramework.LatestCommitment()
			require.NoError(t, err)
			require.Equal(t, epoch.Index(2), ecRecord.EI())
			testFramework.CreateMessage("Message4", tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
			testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

			msg := testFramework.Message("Message4")
			assert.Equal(t, epoch.Index(2), msg.EI())
		}
		// Message5 TX1, issuing time epoch 3
		{
			time.Sleep(epochInterval)
			fmt.Println("message 5")

			eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))

			ecRecord, _, err := testFramework.LatestCommitment()
			require.NoError(t, err)
			require.Equal(t, epoch.Index(3), ecRecord.EI())
			testFramework.CreateMessage("Message5", tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
			testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

			msg := testFramework.Message("Message5")
			assert.Equal(t, epoch.Index(3), msg.EI())
		}
		// Message6 TX2, issuing time epoch 3
		{
			fmt.Println("message 6")

			eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))

			ecRecord, _, err := testFramework.LatestCommitment()
			require.NoError(t, err)
			require.Equal(t, epoch.Index(3), ecRecord.EI())
			testFramework.CreateMessage("Message6", tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["E"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D", 500), tangle.WithECRecord(ecRecord))
			testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

			msg := testFramework.Message("Message6")
			assert.Equal(t, epoch.Index(3), msg.EI())
		}

		// Message7, issuing time epoch 4
		{
			time.Sleep(epochInterval)
			fmt.Println("message 7")

			eventHandlerMock.Expect("EpochCommitted", epoch.Index(4))

			ecRecord, _, err := testFramework.LatestCommitment()
			require.NoError(t, err)
			require.Equal(t, epoch.Index(4), ecRecord.EI())
			testFramework.CreateMessage("Message7", tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
			testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

			msg := testFramework.Message("Message7")
			assert.Equal(t, epoch.Index(4), msg.EI())
			assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
				"Message1": true,
				"Message2": true,
				"Message3": true,
				"Message4": true,
				"Message5": true,
				"Message6": false,
				"Message7": false,
			})
			assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
				"Message5": true,
				"Message6": false,
			})
		}

		eventHandlerMock.AssertExpectations(t)
	*/
}

func setupFramework(t *testing.T, epochInterval time.Duration, options ...tangle.Option) (testFramework *tangle.MessageTestFramework, eventMock *EventMock, m *Manager) {
	var minCommittable time.Duration = 2 * epochInterval
	genesisTime := time.Now()

	testTangle := tangle.NewTestTangle(options...)
	testTangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 2

	testFramework = tangle.NewMessageTestFramework(testTangle, tangle.WithGenesisOutput("A", 500), tangle.WithGenesisOutput("B", 500))

	// set up finality gadget
	testOpts := []finality.Option{
		finality.WithBranchThresholdTranslation(TestBranchGoFTranslation),
		finality.WithMessageThresholdTranslation(TestMessageGoFTranslation),
	}
	sfg := finality.NewSimpleFinalityGadget(testTangle, testOpts...)
	testTangle.ConfirmationOracle = sfg

	// set up notarization manager
	ecFactory := NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 0)
	epochMgr := NewEpochManager(Duration(epochInterval), GenesisTime(genesisTime.Unix()))
	m = NewManager(epochMgr, ecFactory, testTangle, MinCommittableEpochAge(minCommittable))

	commitmentFunc := func() (ecRecord *epoch.ECRecord, latestConfirmedEpoch epoch.Index, err error) {
		ecRecord, err = m.GetLatestEC()
		require.NoError(t, err)
		latestConfirmedEpoch, err = m.LatestConfirmedEpochIndex()
		require.NoError(t, err)
		return ecRecord, latestConfirmedEpoch, nil
	}
	testTangle.Options.CommitmentFunc = commitmentFunc

	testTangle.Setup()
	registerToTangleEvents(sfg, testTangle)
	loadSnapshot(m, testFramework)

	eventMock = NewEventMock(t, m, ecFactory)

	return testFramework, eventMock, m
}

func assertDiffStores(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[epoch.Index][]string) {
	for ei, aliases := range results {
		expectedSpent := make([]*ledger.OutputWithMetadata, 0)
		expectedCreated := make([]*ledger.OutputWithMetadata, 0)
		tmpCreated := make([]*ledger.OutputWithMetadata, 0)

		for _, a := range aliases {
			s, c := m.resolveOutputs(testFramework.Transaction(a))
			tmpCreated = append(tmpCreated, c...)
			expectedSpent = append(expectedSpent, s...)
		}
		// remove created are spent in the same epoch
		for _, c := range tmpCreated {
			spent := false
			for _, s := range expectedSpent {
				if c.ID() == s.ID() {
					spent = true
					break
				}
			}
			if !spent {
				expectedCreated = append(expectedCreated, c)
			}
		}

		actualSpent, actualCreated := m.epochCommitmentFactory.loadDiffUTXOs(ei)
		assert.Equal(t, len(expectedCreated), len(actualCreated))
		assert.Equal(t, len(expectedSpent), len(actualSpent))
	}
}

func assertExistenceOfBlock(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[string]bool) {
	for alias, result := range results {
		msgID := testFramework.Message(alias).ID()
		p, err := m.GetBlockInclusionProof(msgID)
		require.NoError(t, err)
		var ei epoch.Index
		m.tangle.Storage.Message(msgID).Consume(func(block *tangle.Message) {
			t := block.IssuingTime()
			ei = m.epochManager.TimeToEI(t)
		})
		valid := m.epochCommitmentFactory.VerifyTangleRoot(*p, msgID)
		assert.Equal(t, result, valid, "block %s not included in epoch %s", alias, ei)
	}
}

func assertExistenceOfTransaction(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[string]bool) {
	for alias, result := range results {
		txID := testFramework.Transaction(alias).ID()
		p, err := m.GetTransactionInclusionProof(txID)
		require.NoError(t, err)
		var ei epoch.Index
		m.tangle.Ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
			t := tx.(*devnetvm.Transaction).Essence().Timestamp()
			ei = m.epochManager.TimeToEI(t)
		})
		valid := m.epochCommitmentFactory.VerifyStateMutationRoot(*p, testFramework.TransactionID(alias))
		assert.Equal(t, result, valid, "transaction %s not included in epoch %s", alias, ei)
	}
}

func assertEpochDiff(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, ei epoch.Index, expectedSpentAliases, expectedCreatedAliases []string) {
	spent, created := m.epochCommitmentFactory.loadDiffUTXOs(ei)
	expectedSpentIDs := utxo.NewOutputIDs()
	expectedCreatedIDs := utxo.NewOutputIDs()
	actualSpentIDs := utxo.NewOutputIDs()
	actualCreatedIDs := utxo.NewOutputIDs()

	for _, alias := range expectedSpentAliases {
		expectedSpentIDs.Add(testFramework.Output(alias).ID())
	}

	for _, alias := range expectedCreatedAliases {
		expectedCreatedIDs.Add(testFramework.Output(alias).ID())
	}

	for _, outputWithMetadata := range spent {
		actualSpentIDs.Add(outputWithMetadata.ID())
	}

	for _, outputWithMetadata := range created {
		actualCreatedIDs.Add(outputWithMetadata.ID())
	}

	assert.True(t, expectedSpentIDs.Equal(actualSpentIDs), "spent outputs for epoch %d do not match:\nExpected: %v\nActual: %v", ei, expectedSpentIDs, actualSpentIDs)
	assert.True(t, expectedCreatedIDs.Equal(actualCreatedIDs), "created outputs for epoch %d do not match:\nExpected: %v\nActual: %v", ei, expectedCreatedIDs, actualCreatedIDs)
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	testTangle := tangle.NewTestTangle()
	interval := 5 * time.Minute
	return NewManager(NewEpochManager(GenesisTime(t), Duration(interval)), NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 0), testTangle, MinCommittableEpochAge(10*time.Minute))
}

func loadSnapshot(m *Manager, testFramework *tangle.MessageTestFramework) {
	snapshot := testFramework.Snapshot()
	snapshot.DiffEpochIndex = epoch.Index(0)
	snapshot.FullEpochIndex = epoch.Index(0)

	var createMetadata []*ledger.OutputWithMetadata
	for _, metadata := range snapshot.OutputsWithMetadata {
		createMetadata = append(createMetadata, metadata)
	}
	snapshot.EpochDiffs = make(map[epoch.Index]*ledger.EpochDiff)
	snapshot.EpochDiffs[epoch.Index(0)] = ledger.NewEpochDiff([]*ledger.OutputWithMetadata{}, createMetadata)

	ecRecord := epoch.NewECRecord(snapshot.FullEpochIndex)
	ecRecord.SetECR(epoch.MerkleRoot{})
	ecRecord.SetPrevEC(epoch.MerkleRoot{})
	snapshot.LatestECRecord = ecRecord

	m.LoadSnapshot(snapshot)
}

func registerToTangleEvents(sfg *finality.SimpleFinalityGadget, testTangle *tangle.Tangle) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		sfg.HandleMarker(e.Marker, e.Weight)
	}))
	testTangle.ApprovalWeightManager.Events.BranchWeightChanged.Hook(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		sfg.HandleBranch(e.BranchID, e.Weight)
	}))
}
