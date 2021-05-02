package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/epochs"
)

func TestBucketMessageIDMarshalling(t *testing.T) {
	bucketMessageID := NewBucketMessageID(1234, randomMessageID())

	bucketMessageIDFromBytes, _, err := BucketMessageIDFromBytes(bucketMessageID.Bytes())
	require.NoError(t, err)

	assert.Equal(t, bucketMessageID.Bytes(), bucketMessageIDFromBytes.Bytes())
	assert.Equal(t, bucketMessageID.BucketTime(), bucketMessageIDFromBytes.BucketTime())
	assert.Equal(t, bucketMessageID.MessageID(), bucketMessageIDFromBytes.MessageID())
}

func TestOrderer_Order(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			identity.NewID(keyPair.PublicKey): 100,
		}
	}
	manager := epochs.NewManager(epochs.ManaRetriever(manaRetrieverMock), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
	)
	tangle.Setup()

	time1 := clock.SyncedTime().Add(-18 * time.Minute)
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuingTime(time1), WithIssuer(keyPair.PublicKey))
		testFramework.IssueMessages("Message1").WaitApprovalWeightProcessed()

		assert.True(t, testFramework.MessageMetadata("Message1").IsBooked())
		assert.Equal(t, time1, tangle.TimeManager.Time())
	}

	// issue message in time
	time2 := time1.Add(2 * time.Minute)
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuingTime(time2), WithIssuer(keyPair.PublicKey))
		testFramework.IssueMessages("Message2").WaitApprovalWeightProcessed()

		assert.True(t, testFramework.MessageMetadata("Message2").IsBooked())
		assert.Equal(t, time2, tangle.TimeManager.Time())
	}

	// issue message too far in the future of TangleTime
	time3 := time2.Add(11*time.Minute + 20*time.Second)
	{
		testFramework.PreventNewMarkers(true)
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuingTime(time3), WithIssuer(keyPair.PublicKey))
		testFramework.IssueMessages("Message3")
		testFramework.PreventNewMarkers(false)

		assert.False(t, testFramework.MessageMetadata("Message3").IsBooked())
		assert.Equal(t, time2, tangle.TimeManager.Time())

		expectedBucketTime := bucketTime(time3)
		cachedBucketMessageIDs := tangle.Storage.BucketMessageIDs(expectedBucketTime)
		assert.Len(t, cachedBucketMessageIDs, 1)

		cachedBucketMessageIDs.Consume(func(bucketMessageID *BucketMessageID) {
			assert.Equal(t, bucketMessageID.BucketTime(), expectedBucketTime)
			assert.Equal(t, bucketMessageID.MessageID(), testFramework.MessageMetadata("Message3").ID())
		})
	}

	// issue message in time, advance TangleTime and then Message3 should be scheduled too
	time4 := time2.Add(2 * time.Minute)
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message2"), WithIssuingTime(time4), WithIssuer(keyPair.PublicKey))
		testFramework.IssueMessages("Message4").WaitApprovalWeightProcessed()

		assert.True(t, testFramework.MessageMetadata("Message4").IsBooked())
		assert.Equal(t, time4, tangle.TimeManager.Time())
	}

	// Message3 should be booked now.
	assert.Eventuallyf(t, func() bool {
		return testFramework.MessageMetadata("Message3").IsBooked()
	}, 10*time.Second, 500*time.Millisecond, "Message %s not booked in time.", testFramework.MessageMetadata("Message3").ID())
}

func TestOrderer(t *testing.T) {
	nodes := map[string]ed25519.PublicKey{
		"A": ed25519.GenerateKeyPair().PublicKey,
		"B": ed25519.GenerateKeyPair().PublicKey,
	}

	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			identity.NewID(nodes["A"]): 33,
			identity.NewID(nodes["B"]): 67,
		}
	}
	manager := epochs.NewManager(epochs.ManaRetriever(manaRetrieverMock), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)), GenesisNode(nodes["B"].String()))
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
	)
	tangle.Setup()

	time1 := time.Unix(1618034400, 0) // 2021-04-10 06:00:00
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuingTime(time1), WithIssuer(nodes["B"]))
		testFramework.IssueMessages("Message1").WaitApprovalWeightProcessed()

		assert.True(t, testFramework.MessageMetadata("Message1").IsBooked())
		assert.Equal(t, time1, tangle.TimeManager.Time())
	}

	// issue chain of messages (not enough approval weight)
	time2 := time1.Add(9 * time.Minute)
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuingTime(time2), WithIssuer(nodes["A"]))
		testFramework.IssueMessages("Message2").WaitApprovalWeightProcessed()

		assert.True(t, testFramework.MessageMetadata("Message2").IsBooked())
		assert.Equal(t, time1, tangle.TimeManager.Time())
	}
	{
		testFramework.PreventNewMarkers(true)
		time3 := time2.Add(12 * time.Minute)
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuingTime(time3), WithIssuer(nodes["A"]))
		testFramework.IssueMessages("Message3")
		testFramework.PreventNewMarkers(false)

		assert.False(t, testFramework.MessageMetadata("Message3").IsBooked())
		assert.Equal(t, time1, tangle.TimeManager.Time())
	}

	time4 := time2.Add(59 * time.Second)
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message1", "Message2"), WithIssuingTime(time4), WithIssuer(nodes["B"]))
		testFramework.IssueMessages("Message4")

		assert.Eventuallyf(t, func() bool {
			return testFramework.MessageMetadata("Message4").IsBooked()
		}, 5*time.Second, 200*time.Millisecond, "Message4 not booked in time.")
		assert.Equal(t, time4, tangle.TimeManager.Time())
	}

	// Message3 should now be booked too
	time5 := time4.Add(5 * time.Minute)
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithIssuingTime(time5), WithIssuer(nodes["B"]))
		testFramework.IssueMessages("Message5").WaitApprovalWeightProcessed()

		assert.True(t, testFramework.MessageMetadata("Message3").IsBooked())
		assert.True(t, testFramework.MessageMetadata("Message5").IsBooked())
		assert.Equal(t, time5, tangle.TimeManager.Time())
	}
}
