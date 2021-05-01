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
	time3 := time2.Add(12 * time.Minute)
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuingTime(time3), WithIssuer(keyPair.PublicKey))
		testFramework.IssueMessages("Message3")

		assert.False(t, testFramework.MessageMetadata("Message3").IsBooked())
		assert.Equal(t, time2, tangle.TimeManager.Time())

		expectedBucketTime := tangle.Orderer.bucketTime(time3)
		cachedBucketMessageIDs := tangle.Storage.BucketMessageIDs(expectedBucketTime)
		assert.Len(t, cachedBucketMessageIDs, 1)

		cachedBucketMessageIDs.Consume(func(bucketMessageID *BucketMessageID) {
			assert.Equal(t, bucketMessageID.BucketTime(), expectedBucketTime)
			assert.Equal(t, bucketMessageID.MessageID(), testFramework.MessageMetadata("Message3").ID())
		})

	}
}
