package tangle

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/serix"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func TestSerixMessage(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(NewParentMessageIDs().AddStrong(EmptyMessageID), time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)

	// fmt.Println(msg)

	result, err := serix.DefaultAPI.Encode(context.Background(), msg)
	assert.NoError(t, err)

	fmt.Println("Bytes", len(msg.Bytes()), msg.Bytes())
	fmt.Println("Serix", len(result), result)

	assert.Equal(t, msg.Bytes(), result)
}

func TestSerixMarkerMessageMapping(t *testing.T) {
	obj := NewMarkerMessageMapping(markers.NewMarker(1, 5), randomMessageID())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.Marker())
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)
}

func TestSerixBranchWeight(t *testing.T) {
	obj := NewBranchWeight(ledgerstate.BranchIDFromRandomness())
	obj.SetWeight(0.65)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.BranchID())
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)
}

func TestSerixLatestMarkerVotes(t *testing.T) {
	obj := NewLatestMarkerVotes(1, identity.GenerateLocalIdentity().ID())

	serixBytesSeq, err := serix.DefaultAPI.Encode(context.Background(), obj.SequenceID)
	assert.NoError(t, err)
	serixBytesVoter, err := serix.DefaultAPI.Encode(context.Background(), obj.Voter())
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), byteutils.ConcatBytes(serixBytesSeq, serixBytesVoter))

	// TODO: thresholdmap serialization
	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixLatestBranchVotes(t *testing.T) {
	obj := NewLatestBranchVotes(identity.GenerateLocalIdentity().ID())
	obj.Store(new(BranchVote).WithBranchID(ledgerstate.BranchIDFromRandomness()).WithOpinion(Confirmed))

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.Voter)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixBranchVoters(t *testing.T) {
	obj := NewBranchVoters(ledgerstate.BranchIDFromRandomness())
	voters := NewVoters()
	voters.Add(identity.GenerateLocalIdentity().ID())
	voters.Add(identity.GenerateLocalIdentity().ID())
	obj.AddVoters(voters)
	// TODO fix when set can be serialized
	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.BranchID())
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixMarkerIndexBranchIDMapping(t *testing.T) {
	obj := NewMarkerIndexBranchIDMapping(1)
	obj.SetBranchIDs(3, ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness(), ledgerstate.BranchIDFromRandomness()))

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.SequenceID())
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	// TODO threshold map needs to be serialized
	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixAttachment(t *testing.T) {
	obj := NewAttachment(randomTransaction().ID(), randomMessageID())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytes)
}

func TestSerixMissingMessage(t *testing.T) {
	obj := NewMissingMessage(randomMessageID())

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.MessageID())
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixApprover(t *testing.T) {
	obj := NewApprover(StrongApprover, randomMessageID(), randomMessageID())

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)
}

func TestSerixMessageMetadata(t *testing.T) {
	obj := NewMessageMetadata(randomMessageID())
	obj.SetGradeOfFinality(gof.High)
	obj.SetSolid(true)
	obj.SetAddedBranchIDs(ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness(), ledgerstate.BranchIDFromRandomness()))
	obj.SetQueuedTime(time.Now())
	obj.SetScheduled(true)
	obj.SetBooked(true)
	obj.SetStructureDetails(&markers.StructureDetails{
		PastMarkers:   markers.NewMarkers(),
		FutureMarkers: markers.NewMarkers(),
	})

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)
}
