package notarization

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"

	"github.com/iotaledger/goshimmer/packages/core/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	testTangle := tangleold.NewTestTangle()
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
//	testFramework.CreateBlock("Block7", tangle.WithIssuingTime(genesisTime.Add(epochDuration*6)), tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))
//	testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
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
	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		ei := epoch.IndexFromTime(time.Now())
		weightProvider.Update(ei, nodes["A"].ID())
		return map[identity.ID]float64{
			nodes["A"].ID(): 100,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

	genesisTime := time.Now().Add(-25 * time.Minute)
	epochDuration := 5 * time.Minute

	testFramework, eventHandlerMock, m := setupFramework(t, genesisTime, epochDuration, epochDuration*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	ecRecord, _, err := testFramework.LatestCommitment()
	require.NoError(t, err)

	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 1; i <= 5; i++ {
		m.increasePendingConflictCounter(epoch.Index(i))
	}
	// Make all epochs committable by advancing ATT
	testFramework.CreateBlock("Block7", tangleold.WithIssuingTime(genesisTime.Add(epochDuration*6)), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
	testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.Index(0), commitment.EI())

	epochCommittableEvents, manaVectorUpdateEvents := m.decreasePendingConflictCounter(4)
	assert.Emptyf(t, epochCommittableEvents, "expected no epoch committable events")
	assert.Emptyf(t, manaVectorUpdateEvents, "expected no mana vector update events")

	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough and epoch 1 has pbc != 0
	assert.Equal(t, epoch.Index(0), commitment.EI())
	event.Loop.WaitUntilAllTasksProcessed()
	eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
	eventHandlerMock.Expect("EpochCommittable", epoch.Index(2))
	//
	// eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(2))
	committableEvents, _ := m.decreasePendingConflictCounter(1)
	assert.Len(t, committableEvents, 1)
	assert.Equal(t, epoch.Index(1), committableEvents[0].EI)

	committableEvents, _ = m.decreasePendingConflictCounter(2)
	assert.Len(t, committableEvents, 1)
	assert.Equal(t, epoch.Index(2), committableEvents[0].EI)

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

	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 20,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 25,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	var EC0 epoch.EC

	issuingTime := genesisTime

	// Block1, issuing time epoch 1
	{
		fmt.Println("block 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC0 = ecRecord.ComputeEC()
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		testFramework.CreateBlock("Block1", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block1")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block2, issuing time epoch 2
	{
		fmt.Println("block 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		assert.Equal(t, EC0, ecRecord.ComputeEC())
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		testFramework.CreateBlock("Block2", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block1"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block2")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Block1": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Block3, issuing time epoch 3
	{
		fmt.Println("block 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		assert.Equal(t, EC0, ecRecord.ComputeEC())
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		testFramework.CreateBlock("Block3", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block2"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block3")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Block2": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Block4, issuing time epoch 4
	{
		fmt.Println("block 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		assert.Equal(t, EC0, ecRecord.ComputeEC())
		// PrevEC of Epoch0 is the empty Merkle Root
		assert.Equal(t, epoch.MerkleRoot{}, ecRecord.PrevEC())
		event.Loop.WaitUntilAllTasksProcessed()
		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(1))

		testFramework.CreateBlock("Block4", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block3", "Block2"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block4")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Block3": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Block5, issuing time epoch 5
	{
		fmt.Println("block 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		fmt.Println(ecRecord)
		testFramework.CreateBlock("Block5", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block5")
		assert.Equal(t, epoch.Index(1), blk.ECRecordEI())
		assert.Equal(t, EC0, ecRecord.PrevEC())
	}

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_UpdateStateMutationTree(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	var EC0, EC1, EC2 epoch.EC
	issuingTime := genesisTime
	// Block1, issuing time epoch 1
	{
		fmt.Println("block 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC0 = ecRecord.ComputeEC()
		testFramework.CreateBlock("Block1", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block1")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block2, issuing time epoch 2
	{
		fmt.Println("block 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block2", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block1"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block2")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block3, issuing time epoch 3
	{
		fmt.Println("block 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block3", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block2"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block3")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block4, issuing time epoch 4
	{
		fmt.Println("block 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(1))
		testFramework.CreateBlock("Block4", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block3"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block4")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block5 TX1, issuing time epoch 5
	{
		fmt.Println("block 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC1 = ecRecord.ComputeEC()

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(2))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(2))

		testFramework.CreateBlock("Block5", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithInputs("A"), tangleold.WithOutput("C", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block5")
		assert.Equal(t, epoch.Index(1), blk.ECRecordEI())
		assert.Equal(t, EC0, ecRecord.PrevEC())
	}

	// Block6 TX2, issuing time epoch 5
	{
		fmt.Println("block 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		EC2 = ecRecord.ComputeEC()

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(3))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(3))

		testFramework.CreateBlock("Block6", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block5"), tangleold.WithIssuer(nodes["E"].PublicKey()), tangleold.WithInputs("B"), tangleold.WithOutput("D", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block6")
		assert.Equal(t, epoch.Index(2), blk.ECRecordEI())
		assert.Equal(t, EC1, ecRecord.PrevEC())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block7, issuing time epoch 6
	{

		fmt.Println("block 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(4))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(4))
		testFramework.CreateBlock("Block7", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block6"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithInputs("C"), tangleold.WithOutput("E", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block7")
		assert.Equal(t, epoch.Index(3), blk.ECRecordEI())
		assert.Equal(t, EC2, ecRecord.PrevEC())
	}

	// Block8, issuing time epoch 6
	{
		fmt.Println("block 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block8", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block7"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block8")
		assert.Equal(t, epoch.Index(3), blk.ECRecordEI())
		assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
			"Block5": true,
			"Block6": true,
		})
	}

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_UpdateStateMutationTreeWithConflict(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
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
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	issuingTime := genesisTime

	// Block1, issuing time epoch 1
	{
		fmt.Println("block 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block1", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block1")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block2, issuing time epoch 1
	{
		fmt.Println("block 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block2", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block1"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block2")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block3, issuing time epoch 1
	{
		fmt.Println("block 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block3", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block2"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block3")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block4, issuing time epoch 1
	{
		fmt.Println("block 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block4", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block3"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block4")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block5 TX1, issuing time epoch 2
	{
		fmt.Println("block 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block5", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithInputs("A"), tangleold.WithOutput("B", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block5")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block6 TX2, issuing time epoch 2
	{
		fmt.Println("block 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block6", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithInputs("A"), tangleold.WithOutput("C", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block6")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Block1": true,
		"Block2": true,
		"Block3": true,
		"Block4": true,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Block7, issuing time epoch 3
	{
		fmt.Println("block 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block7", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block5"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithInputs("B"), tangleold.WithOutput("E", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block7")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Block5": true,
		"Block6": false,
	})
	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Block5": true,
		"Block6": false,
	})

	issuingTime = issuingTime.Add(epochInterval)

	// Block8, issuing time epoch 4
	{
		fmt.Println("block 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(1))
		testFramework.CreateBlock("Block8", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block7"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block8")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
		"Block7": true,
	})
	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Block7": true,
	})

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_TransactionInclusionUpdate(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
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
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	issuingTime := genesisTime

	// Block1, issuing time epoch 1
	{
		fmt.Println("block 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block1", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block1")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block2, issuing time epoch 1
	{
		fmt.Println("block 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block2", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block1"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block2")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block3 TX1, issuing time epoch 1
	{
		fmt.Println("block 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block3", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block2"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithInputs("A"), tangleold.WithOutput("C", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block3")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block4 TX2, issuing time epoch 1
	{
		fmt.Println("block 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block4", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block2"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithInputs("B"), tangleold.WithOutput("D", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block4")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())

		// pre-create block 8
		testFramework.CreateBlock("Block8", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithInputs("C"), tangleold.WithOutput("E", 500), tangleold.WithECRecord(ecRecord))
	}

	issuingTime = issuingTime.Add(epochInterval)

	// Block5, issuing time epoch 2
	{
		fmt.Println("block 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block5", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block3"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block5")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block6, issuing time epoch 2
	{
		fmt.Println("block 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block6", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block5"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithReattachment("Block8"), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block6")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block7, issuing time epoch 2
	{
		fmt.Println("block 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block7", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block6"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block7")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block8, issuing time epoch 1, earlier attachment of Block6, with same tx
	{
		testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block8")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block9, issuing time epoch 2
	{
		fmt.Println("block 9")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block9", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block8", "Block7"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block9")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}
	// Block10, issuing time epoch 2
	{
		fmt.Println("block 10")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		testFramework.CreateBlock("Block10", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block9"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block10")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
		"Block3": true,
		"Block4": true,
		"Block8": true,
	})

	assertEpochDiff(t, testFramework, notarizationMgr, 1, []string{"A", "B"}, []string{"D", "E"})
	assertEpochDiff(t, testFramework, notarizationMgr, 2, []string{}, []string{})

	// The transaction should be moved to the earlier epoch
	p, err := notarizationMgr.GetTransactionInclusionProof(testFramework.Transaction("Block6").ID())
	require.NoError(t, err)
	assert.Equal(t, epoch.Index(1), p.EI)

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_DiffUTXOs(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	epochInterval := 1 * time.Second

	// Make Current Epoch be epoch 5
	genesisTime := time.Now().Add(-epochInterval * 5)

	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)
	testFramework, eventHandlerMock, notarizationMgr := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	issuingTime := genesisTime

	// Block1, issuing time epoch 1
	{
		fmt.Println("block 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateBlock("Block1", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithInputs("A"), tangleold.WithOutput("C1", 400), tangleold.WithOutput("C1+", 100), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block1")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	// Block2, issuing time epoch 1
	{
		fmt.Println("block 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateBlock("Block2", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block1"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithInputs("B"), tangleold.WithOutput("D2", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block2")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(1), []string{"A"}, []string{"C1", "C1+"})

	issuingTime = issuingTime.Add(epochInterval)

	// Block3, issuing time epoch 2
	{
		fmt.Println("block 3")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateBlock("Block3", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block2"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithInputs("D2"), tangleold.WithOutput("E3", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block3")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	// Block4, issuing time epoch 2
	{
		fmt.Println("block 4")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())
		testFramework.CreateBlock("Block4", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block3"), tangleold.WithIssuer(nodes["D"].PublicKey()), tangleold.WithInputs("E3"), tangleold.WithOutput("F4", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block4")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(2), []string{"D2"}, []string{"E3"})

	issuingTime = issuingTime.Add(epochInterval)

	// Block5, issuing time epoch 3
	{
		fmt.Println("block 5")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())

		testFramework.CreateBlock("Block5", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithInputs("F4"), tangleold.WithOutput("G5", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block5")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(1), []string{"A", "B"}, []string{"C1", "C1+", "D2"})
	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(2), []string{"D2"}, []string{"F4"})

	// Block6, issuing time epoch 3
	{
		fmt.Println("block 6")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(0), ecRecord.EI())

		eventHandlerMock.Expect("EpochCommittable", epoch.Index(1))
		eventHandlerMock.Expect("ManaVectorUpdate", epoch.Index(1))
		testFramework.CreateBlock("Block6", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block5"), tangleold.WithIssuer(nodes["E"].PublicKey()), tangleold.WithInputs("G5"), tangleold.WithOutput("H6", 500), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block6")
		assert.Equal(t, epoch.Index(0), blk.ECRecordEI())
	}

	// Block7, issuing time epoch 3, if we loaded the diff we should just have F4 and H6 as spent and created
	{
		fmt.Println("block 7")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateBlock("Block7", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block6"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block7")
		assert.Equal(t, epoch.Index(1), blk.ECRecordEI())
	}

	// Block8, issuing time epoch 2, reattaches Block6's TX from epoch 3 to epoch 2
	{
		fmt.Println("block 8")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateBlock("Block8", tangleold.WithIssuingTime(issuingTime.Add(-epochInterval)), tangleold.WithStrongParents("Block4"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithReattachment("Block6"), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block8")
		assert.Equal(t, epoch.Index(1), blk.ECRecordEI())
	}

	// Block9, issuing time epoch 3, confirms Block8 (reattachment of Block 6)
	{
		fmt.Println("block 9")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateBlock("Block9", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block8"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block9")
		assert.Equal(t, epoch.Index(1), blk.ECRecordEI())
	}

	// Block10, issuing time epoch 3, confirms Block9 and reattachment of Block 6
	{
		fmt.Println("block 10")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		require.Equal(t, epoch.Index(1), ecRecord.EI())

		testFramework.CreateBlock("Block10", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Block9"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

		blk := testFramework.Block("Block10")
		assert.Equal(t, epoch.Index(1), blk.ECRecordEI())
	}

	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(2), []string{"G5", "D2"}, []string{"F4", "H6"})
	assertEpochDiff(t, testFramework, notarizationMgr, epoch.Index(3), []string{"F4"}, []string{"G5"})

	eventHandlerMock.AssertExpectations(t)
}

func TestManager_ActivityTree(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}
	// Make Current Epoch be epoch 5
	epochInterval := 1 * time.Second
	genesisTime := time.Now().Add(-epochInterval * 5)

	timeManager := struct{ time time.Time }{time: genesisTime}
	timeRetrieverFunc := func() time.Time { return timeManager.time }

	var weightProvider *tangleold.CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	//for _, node := range nodes {
	//	ei := epoch.IndexFromTime(time.Now())
	//	weightProvider.Update(ei, node.ID())
	//}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	weightProvider = tangleold.NewCManaWeightProvider(manaRetrieverMock, timeRetrieverFunc, confirmedRetrieverFunc)
	testFramework, _, _ := setupFramework(t, genesisTime, epochInterval, epochInterval*2, tangleold.ApprovalWeights(weightProvider), tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))

	// expected activity records
	activeNodesTest := make(map[epoch.Index][]identity.ID)
	activeNodesTest[epoch.Index(6)] = []identity.ID{nodes["A"].ID(), nodes["B"].ID()}
	activeNodesTest[epoch.Index(7)] = []identity.ID{nodes["A"].ID(), nodes["B"].ID(), nodes["C"].ID()}
	activeNodesTest[epoch.Index(10)] = []identity.ID{nodes["C"].ID()}

	issuingTime := genesisTime.Add(epochInterval * 5)
	timeManager.time = issuingTime
	// Block1, issuing time epoch 1
	{
		fmt.Println("block 1 and block 2 in epoch 1")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		ei := epoch.IndexFromTime(issuingTime)

		testFramework.CreateBlock("Block1", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["A"].PublicKey()), tangleold.WithInputs("A"), tangleold.WithOutput("C1", 400), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
		testFramework.CreateBlock("Block2", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["B"].PublicKey()), tangleold.WithInputs("B"), tangleold.WithOutput("C2", 400), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
		weightProvider.Update(ei, nodes["A"].ID())
		weightProvider.Update(ei, nodes["B"].ID())

		activeNodes, _ := weightProvider.WeightsOfRelevantVoters()
		assert.Equal(t, len(activeNodesTest[ei]), len(activeNodes))
		for _, n := range activeNodesTest[ei] {
			assert.Contains(t, activeNodes, n)
		}
	}

	issuingTime = issuingTime.Add(epochInterval)
	timeManager.time = issuingTime

	{
		fmt.Println("block 3 in epoch 2")

		ecRecord, _, err := testFramework.LatestCommitment()
		require.NoError(t, err)
		ei := epoch.IndexFromTime(issuingTime)

		testFramework.CreateBlock("Block3", tangleold.WithIssuingTime(issuingTime), tangleold.WithStrongParents("Genesis"), tangleold.WithIssuer(nodes["C"].PublicKey()), tangleold.WithInputs("C1"), tangleold.WithInputs("C2"), tangleold.WithOutput("E1", 800), tangleold.WithECRecord(ecRecord))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()
		weightProvider.Update(epoch.IndexFromTime(issuingTime), nodes["C"].ID())

		activeNodes, _ := weightProvider.WeightsOfRelevantVoters()
		assert.Equal(t, len(activeNodesTest[ei]), len(activeNodes))
		for _, n := range activeNodesTest[ei] {
			assert.Contains(t, activeNodes, n)
		}
	}

	issuingTime = issuingTime.Add(epochInterval * 16)
	timeManager.time = issuingTime

	{
		ei := epoch.IndexFromTime(issuingTime)

		activeNodes, _ := weightProvider.WeightsOfRelevantVoters()
		assert.Equal(t, len(activeNodesTest[ei]), len(activeNodes))
		for _, n := range activeNodesTest[ei] {
			assert.Contains(t, activeNodes, n)
		}
	}

}

func setupFramework(t *testing.T, genesisTime time.Time, epochInterval time.Duration, minCommittable time.Duration, options ...tangleold.Option) (testFramework *tangleold.BlockTestFramework, eventMock *EventMock, m *Manager) {
	epoch.Duration = int64(epochInterval.Seconds())

	testTangle := tangleold.NewTestTangle(append([]tangleold.Option{tangleold.StartSynced(true), tangleold.GenesisTime(genesisTime)}, options...)...)
	testTangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 0

	testFramework = tangleold.NewBlockTestFramework(testTangle, tangleold.WithGenesisOutput("A", 500), tangleold.WithGenesisOutput("B", 500))

	// set up finality gadget
	testOpts := []acceptance.Option{
		acceptance.WithConflictThresholdTranslation(TestConflictAcceptanceStateTranslation),
		acceptance.WithBlockThresholdTranslation(TestBlockAcceptanceStateTranslation),
	}
	sfg := acceptance.NewSimpleFinalityGadget(testTangle, testOpts...)
	testTangle.ConfirmationOracle = sfg

	// set up notarization manager
	ecFactory := NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 0)
	m = NewManager(ecFactory, testTangle, MinCommittableEpochAge(minCommittable), BootstrapWindow(minCommittable*2), Log(logger.NewExampleLogger("test")))

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

	return testFramework, eventMock, m
}

func assertExistenceOfBlock(t *testing.T, testFramework *tangleold.BlockTestFramework, m *Manager, results map[string]bool) {
	event.Loop.WaitUntilAllTasksProcessed()

	for alias, result := range results {
		blkID := testFramework.Block(alias).ID()
		p, err := m.GetBlockInclusionProof(blkID)
		require.NoError(t, err)
		var ei epoch.Index
		m.tangle.Storage.Block(blkID).Consume(func(block *tangleold.Block) {
			t := block.IssuingTime()
			ei = epoch.IndexFromTime(t)
		})
		valid := m.epochCommitmentFactory.VerifyTangleRoot(*p, blkID)
		assert.Equal(t, result, valid, "block %s not included in epoch %s", alias, ei)
	}
}

func assertExistenceOfTransaction(t *testing.T, testFramework *tangleold.BlockTestFramework, m *Manager, results map[string]bool) {
	event.Loop.WaitUntilAllTasksProcessed()

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

func assertEpochDiff(t *testing.T, testFramework *tangleold.BlockTestFramework, m *Manager, ei epoch.Index, expectedSpentAliases, expectedCreatedAliases []string) {
	event.Loop.WaitUntilAllTasksProcessed()

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

func loadSnapshot(m *Manager, testFramework *tangleold.BlockTestFramework) {
	snapshot := testFramework.Snapshot()
	header := &ledger.SnapshotHeader{}
	header.DiffEpochIndex = epoch.Index(0)
	header.FullEpochIndex = epoch.Index(0)

	var createMetadata []*ledger.OutputWithMetadata
	for _, metadata := range snapshot.OutputsWithMetadata {
		createMetadata = append(createMetadata, metadata)
	}
	header.OutputWithMetadataCount = uint64(len(snapshot.OutputsWithMetadata))
	snapshot.EpochDiffs = make(map[epoch.Index]*ledger.EpochDiff)
	snapshot.EpochDiffs[epoch.Index(0)] = ledger.NewEpochDiff([]*ledger.OutputWithMetadata{}, createMetadata)

	ecRecord := epoch.NewECRecord(header.FullEpochIndex)
	ecRecord.SetECR(epoch.MerkleRoot{})
	ecRecord.SetPrevEC(epoch.MerkleRoot{})
	header.LatestECRecord = ecRecord
	snapshot.Header = header

	m.LoadOutputsWithMetadata(snapshot.OutputsWithMetadata)
	m.LoadECandEIs(snapshot.Header)
	m.LoadActivityLogs(snapshot.EpochActiveNodes)
}

func registerToTangleEvents(sfg *acceptance.Gadget, testTangle *tangleold.Tangle) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangleold.MarkerWeightChangedEvent) {
		sfg.HandleMarker(e.Marker, e.Weight)
	}))
	testTangle.ApprovalWeightManager.Events.ConflictWeightChanged.Hook(event.NewClosure(func(e *tangleold.ConflictWeightChangedEvent) {
		sfg.HandleConflict(e.ConflictID, e.Weight)
	}))
}
