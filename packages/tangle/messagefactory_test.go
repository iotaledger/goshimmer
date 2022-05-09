package tangle

import (
	"context"
	"crypto/ed25519"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	targetPOW     = 10
	powTimeout    = 10 * time.Second
	totalMessages = 2000
)

func TestMessageFactory_BuildMessage(t *testing.T) {
	selfLocalIdentity := identity.GenerateLocalIdentity()
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	mockOTV := &SimpleMockOnTangleVoting{}
	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	tangle.MessageFactory = NewMessageFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parents MessageIDs, err error) {
			return NewMessageIDs(EmptyMessageID), nil
		}),
		emptyLikeReferences,
	)
	tangle.MessageFactory.SetTimeout(powTimeout)
	defer tangle.MessageFactory.Shutdown()

	// keep track of sequence numbers
	sequenceNumbers := sync.Map{}

	// attach to event and count
	countEvents := uint64(0)
	tangle.MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *Message) {
		atomic.AddUint64(&countEvents, 1)
	}))

	t.Run("CheckProperties", func(t *testing.T) {
		p := payload.NewGenericDataPayload([]byte("TestCheckProperties"))
		msg, err := tangle.MessageFactory.IssuePayload(p)
		require.NoError(t, err)

		// TODO: approval switch: make test case with weak parents
		assert.NotEmpty(t, msg.ParentsByType(StrongParentType))

		// time in range of 0.1 seconds
		assert.InDelta(t, clock.SyncedTime().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

		// check payload
		assert.Equal(t, p, msg.Payload())

		// check total events and sequence number
		assert.EqualValues(t, 1, countEvents)
		assert.EqualValues(t, 0, msg.SequenceNumber())

		sequenceNumbers.Store(msg.SequenceNumber(), true)
	})

	// create messages in parallel
	t.Run("ParallelCreation", func(t *testing.T) {
		for i := 1; i < totalMessages; i++ {
			t.Run("test", func(t *testing.T) {
				t.Parallel()

				p := payload.NewGenericDataPayload([]byte("TestParallelCreation"))
				msg, err := tangle.MessageFactory.IssuePayload(p)
				require.NoError(t, err)

				// TODO: approval switch: make test case with weak parents
				assert.NotEmpty(t, msg.ParentsByType(StrongParentType))

				// time in range of 0.1 seconds
				assert.InDelta(t, clock.SyncedTime().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

				// check payload
				assert.Equal(t, p, msg.Payload())

				sequenceNumbers.Store(msg.SequenceNumber(), true)
			})
		}
	})

	// check total events and sequence number
	assert.EqualValues(t, totalMessages, countEvents)

	max := uint64(0)
	countSequence := 0
	sequenceNumbers.Range(func(key, value interface{}) bool {
		seq := key.(uint64)
		val := value.(bool)
		if val != true {
			return false
		}

		// check for max sequence number
		if seq > max {
			max = seq
		}
		countSequence++
		return true
	})
	assert.EqualValues(t, totalMessages-1, max)
	assert.EqualValues(t, totalMessages, countSequence)
}

func TestMessageFactory_POW(t *testing.T) {
	mockOTV := &SimpleMockOnTangleVoting{}

	tangle := NewTestTangle()
	defer tangle.Shutdown()
	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	msgFactory := NewMessageFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsMessageIDs MessageIDs, err error) {
			return NewMessageIDs(EmptyMessageID), nil
		}),
		emptyLikeReferences,
	)
	defer msgFactory.Shutdown()

	worker := pow.New(1)

	msgFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return worker.Mine(context.Background(), content, targetPOW)
	}))
	msgFactory.SetTimeout(powTimeout)
	msg, err := msgFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
	require.NoError(t, err)

	msgBytes := msg.Bytes()
	content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]

	zeroes, err := worker.LeadingZerosWithNonce(content, msg.Nonce())
	assert.GreaterOrEqual(t, zeroes, targetPOW)
	assert.NoError(t, err)
}

func TestMessageFactory_PrepareLikedReferences_1(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("O1", 500),
		WithGenesisOutput("O2", 500),
	)

	tangle.Setup()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Logf("Error fired: %v", err)
	}))

	// Message 1
	testFramework.CreateMessage("1", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O3", 500))

	// Message 2
	testFramework.CreateMessage("2", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O5", 500))

	// Message 3
	testFramework.CreateMessage("3", WithStrongParents("Genesis"), WithInputs("O2", "O1"), WithOutput("O4", 1000))
	testFramework.IssueMessages("1", "2", "3").WaitMessagesBooked()

	testFramework.RegisterBranchID("1", "1")
	testFramework.RegisterBranchID("2", "2")
	testFramework.RegisterBranchID("3", "3")

	mockOTV := &SimpleMockOnTangleVoting{
		likedConflictMember: map[ledgerstate.BranchID]LikedConflictMembers{
			testFramework.BranchID("3"): {
				likedBranch:     testFramework.BranchID("2"),
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("1"), testFramework.BranchID("2")),
			},
			testFramework.BranchID("2"): {
				likedBranch:     testFramework.BranchID("2"),
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("1"), testFramework.BranchID("3")),
			},
		},
	}

	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	references, referenceNotPossible, err := PrepareReferences(NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("2").ID()), time.Now(), tangle)

	require.NoError(t, err)

	assert.Equal(t, references[ShallowLikeParentType], MessageIDs{testFramework.Message("2").ID(): types.Void})
	assert.Empty(t, referenceNotPossible)
}

func TestMessageFactory_PrepareLikedReferences_2(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("O1", 500),
		WithGenesisOutput("O2", 500),
	)

	tangle.Setup()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Logf("Error fired: %v", err)
	}))

	// Message 1
	testFramework.CreateMessage("1", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O3", 500), WithIssuingTime(time.Now().Add(5*time.Minute)))

	// Message 2
	testFramework.CreateMessage("2", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O5", 500), WithIssuingTime(time.Now().Add(5*time.Minute)))

	// Message 3
	testFramework.CreateMessage("3", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O4", 500))

	// Message 4
	testFramework.CreateMessage("4", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O6", 500))
	testFramework.IssueMessages("1", "2", "3", "4").WaitMessagesBooked()

	testFramework.RegisterBranchID("1", "1")
	testFramework.RegisterBranchID("2", "2")
	testFramework.RegisterBranchID("3", "3")
	testFramework.RegisterBranchID("4", "4")

	mockOTV := &SimpleMockOnTangleVoting{
		likedConflictMember: map[ledgerstate.BranchID]LikedConflictMembers{
			testFramework.BranchID("1"): {
				likedBranch:     testFramework.BranchID("1"),
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("4")),
			},
			testFramework.BranchID("2"): {
				likedBranch:     testFramework.BranchID("2"),
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("3")),
			},
			testFramework.BranchID("3"): {
				likedBranch:     testFramework.BranchID("2"),
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("2")),
			},
			testFramework.BranchID("4"): {
				likedBranch:     testFramework.BranchID("1"),
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("1")),
			},
		},
	}

	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	// Test first set of parents
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("2").ID()), map[ParentsType]MessageIDs{
		StrongParentType:      NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("2").ID()),
		ShallowLikeParentType: NewMessageIDs(testFramework.Message("2").ID()),
	}, time.Now())

	// Test second set of parents
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("2").ID(), testFramework.Message("1").ID()), map[ParentsType]MessageIDs{
		StrongParentType: NewMessageIDs(testFramework.Message("2").ID(), testFramework.Message("1").ID()),
	}, time.Now())

	// Test third set of parents
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("4").ID()), map[ParentsType]MessageIDs{
		StrongParentType:      NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("4").ID()),
		ShallowLikeParentType: NewMessageIDs(testFramework.Message("1").ID(), testFramework.Message("2").ID()),
	}, time.Now())

	// Test fourth set of parents
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("1").ID(), testFramework.Message("2").ID(), testFramework.Message("3").ID(), testFramework.Message("4").ID()), map[ParentsType]MessageIDs{
		StrongParentType:      NewMessageIDs(testFramework.Message("1").ID(), testFramework.Message("2").ID(), testFramework.Message("3").ID(), testFramework.Message("4").ID()),
		ShallowLikeParentType: NewMessageIDs(testFramework.Message("1").ID(), testFramework.Message("2").ID()),
	}, time.Now())

	// Test empty set of parents
	checkReferences(t, tangle, NewMessageIDs(), map[ParentsType]MessageIDs{}, time.Now(), true)

	// Add reattachment that is older than the original message.
	// Message 5 (reattachment)
	testFramework.CreateMessage("5", WithStrongParents("Genesis"), WithReattachment("1"))
	testFramework.IssueMessages("5").WaitMessagesBooked()

	// Select oldest attachment of the message.
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("4").ID()), map[ParentsType]MessageIDs{
		StrongParentType:      NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("4").ID()),
		ShallowLikeParentType: NewMessageIDs(testFramework.Message("2").ID(), testFramework.Message("5").ID()),
	}, time.Now())

	// Do not return too old like reference: remove strong parent and return it so that it can be removed from the tips.
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("4").ID()), map[ParentsType]MessageIDs{
		StrongParentType:      NewMessageIDs(testFramework.Message("3").ID()),
		ShallowLikeParentType: NewMessageIDs(testFramework.Message("2").ID()),
	}, time.Now().Add(maxParentsTimeDifference))

	// Do not return too old like reference: if there's no other strong parent left, an error should be returned.
	checkReferences(t, tangle, NewMessageIDs(testFramework.Message("4").ID()), map[ParentsType]MessageIDs{
		StrongParentType: NewMessageIDs(),
	}, time.Now().Add(maxParentsTimeDifference), true)
}

// Tests if error is returned when non-existing transaction is tried to be liked.
func TestMessageFactory_PrepareLikedReferences_3(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("O1", 500),
		WithGenesisOutput("O2", 500),
	)

	tangle.Setup()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Logf("Error fired: %v", err)
	}))

	// Message 1
	testFramework.CreateMessage("1", WithStrongParents("Genesis"), WithInputs("O1"), WithOutput("O3", 500))

	// Message 2
	testFramework.CreateMessage("2", WithStrongParents("Genesis"), WithInputs("O2"), WithOutput("O5", 500))

	// Message 3
	testFramework.CreateMessage("3", WithStrongParents("Genesis"), WithInputs("O2", "O1"), WithOutput("O4", 1000))
	testFramework.IssueMessages("1", "2", "3").WaitMessagesBooked()

	testFramework.RegisterBranchID("1", "1")
	testFramework.RegisterBranchID("2", "2")
	testFramework.RegisterBranchID("3", "3")

	nonExistingBranchID := ledgerstate.BranchIDFromRandomness()

	mockOTV := &SimpleMockOnTangleVoting{
		likedConflictMember: map[ledgerstate.BranchID]LikedConflictMembers{
			testFramework.BranchID("3"): {
				likedBranch:     nonExistingBranchID,
				conflictMembers: ledgerstate.NewBranchIDs(testFramework.BranchID("2"), nonExistingBranchID),
			},
		},
	}

	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	_, referenceNotPossible, err := PrepareReferences(NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("2").ID()), time.Now(), tangle)
	require.Error(t, err)
	assert.Equal(t, NewMessageIDs(testFramework.Message("3").ID(), testFramework.Message("2").ID()), referenceNotPossible)
}

func checkReferences(t *testing.T, tangle *Tangle, parents MessageIDs, expectedReferences map[ParentsType]MessageIDs, issuingTime time.Time, errorExpected ...bool) {
	actualReferences, referenceNotPossible, err := PrepareReferences(parents, issuingTime, tangle)
	if len(errorExpected) > 0 && errorExpected[0] {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	// Check for parents whose references can't be set and have to be removed from the tips.
	if !parents.Subtract(expectedReferences[StrongParentType]).Empty() {
		assert.Equalf(t, parents, referenceNotPossible, "references to %s not possible, should be removed from parents %s", referenceNotPossible, parents)
	}

	for _, referenceType := range []ParentsType{StrongParentType, ShallowDislikeParentType, ShallowLikeParentType, WeakParentType} {
		assert.Equalf(t, expectedReferences[referenceType], actualReferences[referenceType], "references type %s do not match: expected %s - actual %s", referenceType, expectedReferences[referenceType], actualReferences[referenceType])
	}
}
