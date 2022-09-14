package blockfactory

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

func TestReferenceProvider_References1(t *testing.T) {
	tf := engine.NewTestFramework(t)

	tf.CreateIdentity("V1", validator.WithWeight(10))
	tf.CreateIdentity("V2", validator.WithWeight(20))

	tf.CreateBlock("Block1", models.WithPayload(tf.CreateTransaction("TX1", 3, "Genesis")), models.WithIssuer(tf.Identity("V1").PublicKey()))
	tf.CreateBlock("Block2", models.WithPayload(tf.CreateTransaction("TX2", 1, "TX1.0")), models.WithIssuer(tf.Identity("V1").PublicKey()))
	tf.CreateBlock("Block3", models.WithPayload(tf.CreateTransaction("TX3", 1, "TX1.1")), models.WithIssuer(tf.Identity("V1").PublicKey()))
	tf.CreateBlock("Block4", models.WithPayload(tf.CreateTransaction("TX4", 1, "TX1.0", "TX1.1")), models.WithIssuer(tf.Identity("V2").PublicKey()))
	tf.IssueBlocks("Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(tf.Engine, func() epoch.Index {
		return 0
	})
	references, err := rp.References(nil, tf.BlockIDs("Block3", "Block4"))
	require.NoError(t, err)

	assert.Equal(t, references[models.ShallowLikeParentType], tf.BlockIDs("Block4"))
	assert.Equal(t, references[models.StrongParentType], tf.BlockIDs("Block3", "Block4"))
}

// func TestBlockFactory_PrepareLikedReferences_2(t *testing.T) {
// 	tangle := NewTestTangle()
// 	defer tangle.Shutdown()
//
// 	testFramework := NewBlockTestFramework(
// 		tangle,
// 		WithGenesisOutput("O1", 500),
// 		WithGenesisOutput("O2", 500),
// 	)
//
// 	tangle.Setup()
//
// 	tangle.Events.Error.Hook(event.NewClosure(func(err error) {
// 		t.Logf("Error fired: %v", err)
// 	}))
//
// 	// Block 1
// 	testFramework.CreateBlock("1", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O3", 500), WithIssuingTime(time.Now().Add(5*time.Minute)))
//
// 	// Block 2
// 	testFramework.CreateBlock("2", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O5", 500), WithIssuingTime(time.Now().Add(5*time.Minute)))
//
// 	// Block 3
// 	testFramework.CreateBlock("3", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O4", 500))
//
// 	// Block 4
// 	testFramework.CreateBlock("4", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O6", 500))
// 	testFramework.IssueBlocks("1", "2", "3", "4").WaitUntilAllTasksProcessed()
//
// 	testFramework.RegisterConflictID("1", "1")
// 	testFramework.RegisterConflictID("2", "2")
// 	testFramework.RegisterConflictID("3", "3")
// 	testFramework.RegisterConflictID("4", "4")
//
// 	mockOTV := &SimpleMockOnTangleVoting{
// 		likedConflictMember: map[utxo.TransactionID]LikedConflictMembers{
// 			testFramework.ConflictID("1"): {
// 				likedConflict:   testFramework.ConflictID("1"),
// 				conflictMembers: set.NewAdvancedSet(testFramework.ConflictID("4")),
// 			},
// 			testFramework.ConflictID("2"): {
// 				likedConflict:   testFramework.ConflictID("2"),
// 				conflictMembers: set.NewAdvancedSet(testFramework.ConflictID("3")),
// 			},
// 			testFramework.ConflictID("3"): {
// 				likedConflict:   testFramework.ConflictID("2"),
// 				conflictMembers: set.NewAdvancedSet(testFramework.ConflictID("2")),
// 			},
// 			testFramework.ConflictID("4"): {
// 				likedConflict:   testFramework.ConflictID("1"),
// 				conflictMembers: set.NewAdvancedSet(testFramework.ConflictID("1")),
// 			},
// 		},
// 	}
//
// 	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)
//
// 	// Test first set of parents
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("2").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType:      NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("2").ID()),
// 		ShallowLikeParentType: NewBlockIDs(testFramework.Block("2").ID()),
// 	}, time.Now())
//
// 	// Test second set of parents
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("2").ID(), testFramework.Block("1").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType: NewBlockIDs(testFramework.Block("2").ID(), testFramework.Block("1").ID()),
// 	}, time.Now())
//
// 	// Test third set of parents
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("4").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType:      NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("4").ID()),
// 		ShallowLikeParentType: NewBlockIDs(testFramework.Block("1").ID(), testFramework.Block("2").ID()),
// 	}, time.Now())
//
// 	// Test fourth set of parents
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("1").ID(), testFramework.Block("2").ID(), testFramework.Block("3").ID(), testFramework.Block("4").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType:      NewBlockIDs(testFramework.Block("1").ID(), testFramework.Block("2").ID(), testFramework.Block("3").ID(), testFramework.Block("4").ID()),
// 		ShallowLikeParentType: NewBlockIDs(testFramework.Block("1").ID(), testFramework.Block("2").ID()),
// 	}, time.Now())
//
// 	// Test empty set of parents
// 	checkReferences(t, tangle, nil, NewBlockIDs(), map[ParentsType]BlockIDs{}, time.Now(), true)
//
// 	// Add reattachment that is older than the original block.
// 	// Block 5 (reattachment)
// 	testFramework.CreateBlock("5", WithStrongParents("Genesis"), WithReattachment("1"))
// 	testFramework.IssueBlocks("5").WaitUntilAllTasksProcessed()
//
// 	// Select oldest attachment of the block.
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("4").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType:      NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("4").ID()),
// 		ShallowLikeParentType: NewBlockIDs(testFramework.Block("2").ID(), testFramework.Block("5").ID()),
// 	}, time.Now())
//
// 	// Do not return too old like reference: remove strong parent.
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("4").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType:      NewBlockIDs(testFramework.Block("3").ID()),
// 		ShallowLikeParentType: NewBlockIDs(testFramework.Block("2").ID()),
// 	}, time.Now().Add(maxParentsTimeDifference))
//
// 	// Do not return too old like reference: if there's no other strong parent left, an error should be returned.
// 	checkReferences(t, tangle, nil, NewBlockIDs(testFramework.Block("4").ID()), map[ParentsType]BlockIDs{
// 		StrongParentType: NewBlockIDs(),
// 	}, time.Now().Add(maxParentsTimeDifference), true)
// }




// // Tests if error is returned when non-existing transaction is tried to be liked.
// func TestBlockFactory_PrepareLikedReferences_3(t *testing.T) {
// 	tangle := NewTestTangle()
//
// 	testFramework := NewBlockTestFramework(
// 		tangle,
// 		WithGenesisOutput("O1", 500),
// 		WithGenesisOutput("O2", 500),
// 	)
//
// 	tangle.Setup()
//
// 	tangle.Events.Error.Hook(event.NewClosure(func(err error) {
// 		t.Logf("Error fired: %v", err)
// 	}))
//
// 	// Block 1
// 	testFramework.CreateBlock("1", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O3", 500))
//
// 	// Block 2
// 	testFramework.CreateBlock("2", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O5", 500))
//
// 	// Block 3
// 	testFramework.CreateBlock("3", WithStrongParents("Genesis"), WithInputs("O2", "O1"), WithOutput("O4", 1000))
// 	testFramework.IssueBlocks("1", "2", "3").WaitUntilAllTasksProcessed()
//
// 	testFramework.RegisterConflictID("1", "1")
// 	testFramework.RegisterConflictID("2", "2")
// 	testFramework.RegisterConflictID("3", "3")
//
// 	nonExistingConflictID := randomConflictID()
//
// 	mockOTV := &SimpleMockOnTangleVoting{
// 		likedConflictMember: map[utxo.TransactionID]LikedConflictMembers{
// 			testFramework.ConflictID("2"): {
// 				likedConflict:   nonExistingConflictID,
// 				conflictMembers: set.NewAdvancedSet(testFramework.ConflictID("3"), nonExistingConflictID),
// 			},
// 			testFramework.ConflictID("3"): {
// 				likedConflict:   nonExistingConflictID,
// 				conflictMembers: set.NewAdvancedSet(testFramework.ConflictID("2"), nonExistingConflictID),
// 			},
// 		},
// 	}
//
// 	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)
//
// 	tangle.OrphanageManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
// 		fmt.Println(event.Block.ID())
// 	}))
//
// 	references, err := tangle.Factory.ReferenceProvider.References(nil, NewBlockIDs(testFramework.Block("3").ID(), testFramework.Block("2").ID()), time.Now())
// 	require.Error(t, err)
// 	assert.True(t, references.IsEmpty())
// }
//
// // Tests if weak references are properly constructed from consumed outputs.
// func TestBlockFactory_WeakReferencesConsumed(t *testing.T) {
// 	tangle := NewTestTangle()
//
// 	testFramework := NewBlockTestFramework(
// 		tangle,
// 		WithGenesisOutput("G1", 500),
// 		WithGenesisOutput("G2", 500),
// 	)
//
// 	tangle.Setup()
//
// 	{
// 		testFramework.CreateBlock("1", WithStrongParents("Genesis"), WithInputs("G1"), WithOutput("O1", 500))
// 		testFramework.CreateBlock("2", WithStrongParents("Genesis"), WithInputs("G2"), WithOutput("O2", 500))
// 		testFramework.CreateBlock("3", WithStrongParents("1", "2"))
//
// 		testFramework.IssueBlocks("1", "2", "3").WaitUntilAllTasksProcessed()
//
// 		checkReferences(t, tangle, testFramework.Block("1").Payload(), testFramework.Block("1").ParentsByType(StrongParentType), map[ParentsType]BlockIDs{
// 			StrongParentType: NewBlockIDs(EmptyBlockID),
// 		}, time.Now())
//
// 		checkReferences(t, tangle, testFramework.Block("2").Payload(), testFramework.Block("2").ParentsByType(StrongParentType), map[ParentsType]BlockIDs{
// 			StrongParentType: NewBlockIDs(EmptyBlockID),
// 		}, time.Now())
//
// 		checkReferences(t, tangle, testFramework.Block("3").Payload(), testFramework.Block("3").ParentsByType(StrongParentType), map[ParentsType]BlockIDs{
// 			StrongParentType: testFramework.BlockIDs("1", "2"),
// 		}, time.Now())
// 	}
//
// 	{
// 		testFramework.CreateBlock("4", WithStrongParents("3"), WithInputs("O1", "O2"), WithOutput("O4", 1000))
// 		testFramework.IssueBlocks("4").WaitUntilAllTasksProcessed()
//
// 		// Select oldest attachment of the block.
// 		checkReferences(t, tangle, testFramework.Block("4").Payload(), testFramework.Block("4").ParentsByType(StrongParentType), map[ParentsType]BlockIDs{
// 			StrongParentType: testFramework.BlockIDs("3"),
// 			WeakParentType:   testFramework.BlockIDs("1", "2"),
// 		}, time.Now())
// 	}
// }
//
// func checkReferences(t *testing.T, tangle *Tangle, payload payload.Payload, parents BlockIDs, expectedReferences map[ParentsType]BlockIDs, issuingTime time.Time, errorExpected ...bool) {
// 	actualReferences, err := tangle.Factory.ReferenceProvider.References(payload, parents, issuingTime)
// 	if len(errorExpected) > 0 && errorExpected[0] {
// 		require.Error(t, err)
// 		return
// 	}
// 	require.NoError(t, err)
//
// 	for _, referenceType := range []ParentsType{StrongParentType, ShallowLikeParentType, WeakParentType} {
// 		assert.Equalf(t, expectedReferences[referenceType], actualReferences[referenceType], "references type %s do not match: expected %s - actual %s", referenceType, expectedReferences[referenceType], actualReferences[referenceType])
// 	}
// }
