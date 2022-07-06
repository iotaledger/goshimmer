package notarization

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	m := NewManager(NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 1), testTangle)
	assert.NotNil(t, m)
}

//
// func TestManager_IsCommittable(t *testing.T) {
//	nodes := map[string]*identity.Identity{
//		"A": identity.GenerateIdentity(),
//	}
//	var weightProvider *tangle.CManaWeightProvider
//	manaRetrieverMock := func() map[identity.ID]float64 {
//		weightProvider.Update(time.Now(), nodes["A"].ID())
//		return map[identity.ID]float64{
//			nodes["A"].ID(): 100,
//		}
//	}
//	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
//
//	genesisTime := time.Now().Add(-25 * time.Minute)
//	epochDuration := 5 * time.Minute
//
//	testFramework, eventHandlerMock, m := setupFramework(t, genesisTime, epochDuration, epochDuration*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
//
//	ecRecord, _, err := testFramework.LatestCommitment()
//	require.NoError(t, err)
//
//
//	for i := 1; i < 5; i++ {
//		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
//	}
//
//	// Make all epochs committable by advancing ATT
//	testFramework.CreateMessage("Message7", tangle.WithIssuingTime(genesisTime.Add(epochDuration*6)), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
//	testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()
//
//	ei := epoch.Index(5)
//	m.pendingConflictsCounters[ei] = 0
//	// not old enough
//	assert.False(t, m.isCommittable(ei))
//
//	ei = epoch.Index(1)
//	m.pendingConflictsCounters[ei] = 1
//	// old enough but pbc > 0
//	assert.False(t, m.isCommittable(ei))
//	m.pendingConflictsCounters[ei] = 0
//	// old enough and pbc > 0
//	assert.True(t, m.isCommittable(ei))
// }

func TestManager_GetLatestEC(t *testing.T) {
	nodes := map[string]*identity.Identity{
		"A": identity.GenerateIdentity(),
	}
	var weightProvider *tangle.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		weightProvider.Update(time.Now(), nodes["A"].ID())
		return map[identity.ID]float64{
			nodes["A"].ID(): 100,
		}
	}
	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)

	genesisTime := time.Now().Add(-25 * time.Minute)
	epochDuration := 5 * time.Minute

	testFramework, eventHandlerMock, m := setupFramework(t, genesisTime, epochDuration, epochDuration*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	ecRecord, _, err := testFramework.LatestCommitment()
	require.NoError(t, err)

	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 1; i <= 5; i++ {
		m.increasePendingConflictCounter(epoch.Index(i))
	}
	// Make all epochs committable by advancing ATT
	testFramework.CreateMessage("Message7", tangle.WithIssuingTime(genesisTime.Add(epochDuration*6)), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
	testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.Index(0), commitment.EI())

	m.decreasePendingConflictCounter(4)

	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough and epoch 1 has pbc != 0
	assert.Equal(t, epoch.Index(0), commitment.EI())

	eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
	eventHandlerMock.Expect("EpochCommittable", epoch.Index(2))
	//
	// eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(2))
	m.decreasePendingConflictCounter(1)
	m.decreasePendingConflictCounter(2)
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.Index(2), commitment.EI())
}

func TestManager_UpdateTangleTree(t *testing.T) {
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

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	var EC0 epoch.EC

	issuingTime := genesisTime.Add(epochInterval)

	// Message1, issuing time epoch 1
	{
		fmt.Println("message 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC0 = EC(ecRecord)
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		testFramework.CreateMessage("Message1", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message2, issuing time epoch 2
	{
		fmt.Println("message 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		assert.Equal(t, EC0, EC(ecRecord))
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		testFramework.CreateMessage("Message2", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Message1": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Message3, issuing time epoch 3
	{
		fmt.Println("message 3")
		fmt.Println("issueing time msg 3 ", issuingTime.String())

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		assert.Equal(t, EC0, EC(ecRecord))
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		testFramework.CreateMessage("Message3", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Message2": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Message4, issuing time epoch 4
	{
		fmt.Println("message 4")
		fmt.Println("issuing time msg 4 ", issuingTime.String())
		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		assert.Equal(t, EC0, EC(ecRecord))
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		testFramework.CreateMessage("Message4", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message3", "Message2"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Message3": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Message5, issuing time epoch 5
	{
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		fmt.Println(ecRecord)
		testFramework.CreateMessage("Message5", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(1), msg.EI())
		assert.Equal(t, EC0, ecRecord.PrevEC())
	}

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_UpdateStateMutationTree(t *testing.T) {
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

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	var EC0, EC1, EC2 epoch.EC
	issuingTime := genesisTime.Add(epochInterval)
	// Message1, issuing time epoch 1
	{
		fmt.Println("message 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC0 = EC(ecRecord)
		testFramework.CreateMessage("Message1", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message2, issuing time epoch 2
	{
		fmt.Println("message 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message3, issuing time epoch 3
	{
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message4, issuing time epoch 4
	{
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		testFramework.CreateMessage("Message4", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message5 TX1, issuing time epoch 5
	{
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC1 = EC(ecRecord)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(2))
		testFramework.CreateMessage("Message5", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(1), msg.EI())
		assert.Equal(t, EC0, ecRecord.PrevEC())
	}

	// Message6 TX2, issuing time epoch 5
	{
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC2 = EC(ecRecord)
		eventHandlerMock.Expect("EpochCommittable", epoch.Index(3))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(3), []*ledger.OutputWithMetadata{}, []*ledger.OutputWithMetadata{})
		testFramework.CreateMessage("Message6", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["E"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(2), msg.EI())
		assert.Equal(t, EC1, ecRecord.PrevEC())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message7, issuing time epoch 6
	{

		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(4))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(4), []*ledger.OutputWithMetadata{}, []*ledger.OutputWithMetadata{})
		testFramework.CreateMessage("Message7", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message6"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("C"), tangle.WithOutput("E", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(3), msg.EI())
		assert.Equal(t, EC2, ecRecord.PrevEC())
	}

	// Message8, issuing time epoch 6
	{
		fmt.Println("message 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message8", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message7"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(3), msg.EI())
		assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
			"Message5": true,
			"Message6": true,
		})
	}

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_UpdateStateMutationTreeWithConflict(t *testing.T) {
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

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	issuingTime := genesisTime.Add(epochInterval)

	// Message1, issuing time epoch 1
	{
		fmt.Println("message 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message1", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message2, issuing time epoch 1
	{
		fmt.Println("message 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message3, issuing time epoch 1
	{
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message4, issuing time epoch 1
	{
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message4", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message5 TX1, issuing time epoch 2
	{
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message5", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("B", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message6 TX2, issuing time epoch 2
	{
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message6", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Message1": true,
		"Message2": true,
		"Message3": true,
		"Message4": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Message7, issuing time epoch 3
	{
		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message7", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("E", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Message5": true,
		"Message6": false,
	})
	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Message5": true,
		"Message6": false,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Message8, issuing time epoch 4
	{
		fmt.Println("message 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		testFramework.CreateMessage("Message8", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message7"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Message7": true,
	})
	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Message7": true,
	})

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_TransactionInclusionUpdate(t *testing.T) {
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

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	issuingTime := genesisTime.Add(epochInterval)

	// Message1, issuing time epoch 1
	{
		fmt.Println("message 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message1", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message1")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message2, issuing time epoch 1
	{
		fmt.Println("message 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message2", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message3 TX1, issuing time epoch 1
	{
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message3", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message4 TX2, issuing time epoch 1
	{
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message4", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())

		// pre-create message 8
		testFramework.CreateMessage("Message8", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithInputs("C"), tangle.WithOutput("E", 500), tangle.WithECRecord(ecRecord))
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Message5, issuing time epoch 2
	{
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message5", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message6, issuing time epoch 2
	{
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message6", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithReattachment("Message8"), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message7, issuing time epoch 2
	{
		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message7", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message6"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message8, issuing time epoch 1, earlier attachment of Message6, with same tx
	{
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message9, issuing time epoch 2
	{
		fmt.Println("message 9")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message9", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message8", "Message7"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message9").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message9")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}
	// Message10, issuing time epoch 2
	{
		fmt.Println("message 10")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateMessage("Message10", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message9"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message10").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message10")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Message3": true,
		"Message4": true,
		"Message8": true,
	})

	assertEpochDiff(t, testFramework, notarizationMgr, 1, []string{"A", "B"}, []string{"D", "E"})
	assertEpochDiff(t, testFramework, notarizationMgr, 2, []string{}, []string{})

	// The transaction should be moved to the earlier epoch
	p, err := notarizationMgr.GetTransactionInclusionProof(testFramework.Transaction("Message6").ID())
	require.NoError(t, err)
	assert.Equal(t, epoch.Index(1), p.EI)

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_DiffUTXOs(t *testing.T) {
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

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	weightProvider = tangle.NewCManaWeightProvider(manaRetrieverMock, time.Now)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangle.ApprovalWeights(weightProvider), tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	issuingTime := genesisTime.Add(epochInterval)

	// Message1, issuing time epoch 1
	{
		fmt.Println("message 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message1", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("A"), tangle.WithOutput("C1", 400), tangle.WithOutput("C1+", 100), tangle.WithECRecord(ecRecord))
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
		testFramework.CreateMessage("Message2", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message1"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithInputs("B"), tangle.WithOutput("D2", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message2")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(1), []string{"A"}, []string{"C1", "C1+"})

	issuingTime = issuingTime.Add(epochInterval)

	// Message3, issuing time epoch 2
	{
		fmt.Println("message 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message3", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message2"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithInputs("D2"), tangle.WithOutput("E3", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message3")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	// Message4, issuing time epoch 2
	{
		fmt.Println("message 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateMessage("Message4", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message3"), tangle.WithIssuer(nodes["D"].PublicKey()), tangle.WithInputs("E3"), tangle.WithOutput("F4", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message4")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(2), []string{"D2"}, []string{"E3"})

	issuingTime = issuingTime.Add(epochInterval)

	// Message5, issuing time epoch 3
	{
		fmt.Println("message 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())

		testFramework.CreateMessage("Message5", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithInputs("F4"), tangle.WithOutput("G5", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message5")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(1), []string{"A", "B"}, []string{"C1", "C1+", "D2"})
	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(2), []string{"D2"}, []string{"F4"})

	// Message6, issuing time epoch 3
	{
		fmt.Println("message 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		testFramework.CreateMessage("Message6", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message5"), tangle.WithIssuer(nodes["E"].PublicKey()), tangle.WithInputs("G5"), tangle.WithOutput("H6", 500), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message6")
		assert.Equal(t, epoch.Index(0), msg.EI())
	}

	// Message7, issuing time epoch 3, if we loaded the diff we should just have F4 and H6 as spent and created
	{
		fmt.Println("message 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateMessage("Message7", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message6"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message7")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}

	// Message8, issuing time epoch 2, reattaches Message6's TX from epoch 3 to epoch 2
	{
		fmt.Println("message 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateMessage("Message8", tangle.WithIssuingTime(issuingTime.Add(-epochInterval)), tangle.WithStrongParents("Message4"), tangle.WithIssuer(nodes["B"].PublicKey()), tangle.WithReattachment("Message6"), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message8")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}

	// Message9, issuing time epoch 3, confirms Message8 (reattachment of Message 6)
	{
		fmt.Println("message 9")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateMessage("Message9", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message8"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message9").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message9")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}

	// Message10, issuing time epoch 3, confirms Message9 and reattachment of Message 6
	{
		fmt.Println("message 10")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateMessage("Message10", tangle.WithIssuingTime(issuingTime), tangle.WithStrongParents("Message9"), tangle.WithIssuer(nodes["C"].PublicKey()), tangle.WithECRecord(ecRecord))
		testFramework.IssueMessages("Message10").WaitUntilAllTasksProcessed()

		msg := testFramework.Message("Message10")
		assert.Equal(t, epoch.Index(1), msg.EI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(2), []string{"G5", "D2"}, []string{"F4", "H6"})
	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(3), []string{"F4"}, []string{"G5"})

	eventHandlerMock.AssertExpectations(t)
}

func setupFramework(t *testing.T, genesisTime time.Time, epochInterval time.Duration, minCommittable time.Duration, options ...tangle.Option) (testFramework *tangle.MessageTestFramework, eventMock *EventMock, m *Manager) {
	testTangle := tangle.NewTestTangle(append([]tangle.Option{tangle.StartSynced(true)}, options...)...)
	testTangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 0

	testFramework = tangle.NewMessageTestFramework(testTangle, tangle.WithGenesisOutput("A", 500), tangle.WithGenesisOutput("B", 500))

	// set up finality gadget
	testOpts := []acceptance.Option{
		acceptance.WithBranchThresholdTranslation(TestBranchConfirmationStateTranslation),
		acceptance.WithMessageThresholdTranslation(TestMessageConfirmationStateTranslation),
	}
	sfg := acceptance.NewSimpleFinalityGadget(testTangle, testOpts...)
	testTangle.ConfirmationOracle = sfg

	// set up notarization manager
	ecFactory := NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 0)
	m = NewManager(ecFactory, testTangle, MinCommittableEpochAge(minCommittable), ManaDelay(2), Log(logger.NewExampleLogger("test")))

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

	eventMock = NewEventMock(t, m)

	epoch.Duration = int64(epochInterval.Seconds())
	epoch.GenesisTime = genesisTime.Unix()

	return testFramework, eventMock, m
}

func assertExistenceOfBlock(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[string]bool) {
	for alias, result := range results {
		msgID := testFramework.Message(alias).ID()
		p, err := m.GetBlockInclusionProof(msgID)
		require.NoError(t, err)
		var ei epoch.Index
		m.tangle.Storage.Message(msgID).Consume(func(block *tangle.Message) {
			t := block.IssuingTime()
			ei = epoch.IndexFromTime(t)
		})
		valid := m.epochCommitmentFactory.VerifyTangleRoot(*p, msgID)
		assert.Equal(t, result, valid, "block %s not included in epoch %s", alias, ei)
	}
}

func assertExistenceOfTransaction(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[string]bool) {
	for alias, result := range results {
		var ei epoch.Index
		var notConfirmed bool

		txID := testFramework.Transaction(alias).ID()

		m.tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMeta *ledger.TransactionMetadata) {
			if txMeta.InclusionTime().IsZero() {
				notConfirmed = true
				return
			}
			ei = epoch.IndexFromTime(txMeta.InclusionTime())
		})

		if notConfirmed {
			assert.Equal(t, result, false, "transaction %s not confirmed", alias)
			return
		}

		p, err := m.GetTransactionInclusionProof(txID)
		require.NoError(t, err)

		valid := m.epochCommitmentFactory.VerifyStateMutationRoot(*p, testFramework.TransactionID(alias))
		assert.Equal(t, result, valid, "transaction %s inclusion differs in epoch %s", alias, ei)
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

	assert.True(t, expectedSpentIDs.Equal(actualSpentIDs), "spent outputs for epoch %d do not match:\nExpected: %s\nActual: %s", ei, expectedSpentIDs, actualSpentIDs)
	assert.True(t, expectedCreatedIDs.Equal(actualCreatedIDs), "created outputs for epoch %d do not match:\nExpected: %s\nActual: %s", ei, expectedCreatedIDs, actualCreatedIDs)
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

func registerToTangleEvents(sfg *acceptance.Gadget, testTangle *tangle.Tangle) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		sfg.HandleMarker(e.Marker, e.Weight)
	}))
	testTangle.ApprovalWeightManager.Events.BranchWeightChanged.Hook(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		sfg.HandleBranch(e.BranchID, e.Weight)
	}))
}
