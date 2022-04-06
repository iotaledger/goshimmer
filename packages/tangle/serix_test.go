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

func TestSerixMessage_Correct(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs().
			AddAll(ShallowLikeParentType, NewMessageIDs(numberMessageID(7), numberMessageID(8))).
			AddAll(ShallowDislikeParentType, NewMessageIDs(numberMessageID(2))).
			AddAll(StrongParentType, NewMessageIDs(numberMessageID(1), numberMessageID(7))).
			AddAll(WeakParentType, NewMessageIDs(numberMessageID(5), numberMessageID(6))),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)
	fmt.Println(msg)

	result, err := serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.NoError(t, err)

	fmt.Println("Bytes", len(msg.Bytes()), msg.Bytes())
	fmt.Println("Serix", len(result), result)

	assert.Equal(t, msg.Bytes(), result)
}

func TestSerixMessage_NoParentBlocks(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs(),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.Error(t, err)

}

func TestSerixMessage_EmptyParentBlock(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs().
			AddAll(ShallowLikeParentType, NewMessageIDs()).
			AddAll(ShallowDislikeParentType, NewMessageIDs(numberMessageID(2))).
			AddAll(StrongParentType, NewMessageIDs(numberMessageID(1), numberMessageID(7))).
			AddAll(WeakParentType, NewMessageIDs(numberMessageID(5), numberMessageID(6))),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)
	fmt.Println(msg)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.Error(t, err)
}

func TestSerixMessage_EmptyStrongParents(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs().
			AddAll(ShallowLikeParentType, NewMessageIDs(numberMessageID(7), numberMessageID(8))).
			AddAll(ShallowDislikeParentType, NewMessageIDs(numberMessageID(2))).
			AddAll(StrongParentType, NewMessageIDs()).
			AddAll(WeakParentType, NewMessageIDs(numberMessageID(5), numberMessageID(6))),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)
	fmt.Println(msg)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.Error(t, err, ErrNoStrongParents)
}

func TestSerixMessage_NoStrongParents(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs().
			AddAll(ShallowLikeParentType, NewMessageIDs(numberMessageID(7), numberMessageID(8))).
			AddAll(ShallowDislikeParentType, NewMessageIDs(numberMessageID(2))).
			AddAll(WeakParentType, NewMessageIDs(numberMessageID(5), numberMessageID(6))),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.Error(t, err, ErrNoStrongParents)
}

func TestSerixMessage_IllegalCrossReferences(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs().
			AddAll(ShallowLikeParentType, NewMessageIDs(numberMessageID(5), numberMessageID(8))).
			AddAll(ShallowDislikeParentType, NewMessageIDs(numberMessageID(2))).
			AddAll(StrongParentType, NewMessageIDs(numberMessageID(1), numberMessageID(7))).
			AddAll(WeakParentType, NewMessageIDs(numberMessageID(5), numberMessageID(6))),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)
	fmt.Println(msg)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.NoError(t, err, ErrConflictingReferenceAcrossBlocks)
}

func TestSerixMessage_LegalCrossReferences(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(
		NewParentMessageIDs().
			AddAll(ShallowLikeParentType, NewMessageIDs(numberMessageID(7), numberMessageID(8))).
			AddAll(ShallowDislikeParentType, NewMessageIDs(numberMessageID(2))).
			AddAll(StrongParentType, NewMessageIDs(numberMessageID(7), numberMessageID(7))).
			AddAll(WeakParentType, NewMessageIDs(numberMessageID(7), numberMessageID(6))),
		time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)
	fmt.Println(msg)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	assert.NoError(t, err)
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
	voters.SerializableSet.Add(identity.GenerateLocalIdentity().ID())
	voters.SerializableSet.Add(identity.GenerateLocalIdentity().ID())
	obj.AddVoters(voters)

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.BranchID())
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixMarkerIndexBranchIDMapping(t *testing.T) {
	obj := NewMarkerIndexBranchIDMapping(1)
	obj.SetBranchIDs(3, ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()))

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.SequenceID())
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

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

func TestSerixMessageMetadata_WithStructureDetails(t *testing.T) {
	obj := NewMessageMetadata(randomMessageID())
	obj.SetGradeOfFinality(gof.High)
	obj.SetSolid(true)
	obj.SetAddedBranchIDs(ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()))
	obj.SetSubtractedBranchIDs(ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()))
	obj.SetQueuedTime(time.Now())
	obj.SetScheduled(true)
	obj.SetBooked(true)
	obj.SetStructureDetails(&markers.StructureDetails{
		PastMarkers:   markers.NewMarkers(),
		FutureMarkers: markers.NewMarkers(),
	})

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.MessageID)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixMessageMetadata_EmptyStructureDetails(t *testing.T) {
	obj := NewMessageMetadata(randomMessageID())
	obj.SetGradeOfFinality(gof.High)
	obj.SetSolid(true)
	obj.SetAddedBranchIDs(ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()))
	obj.SetSubtractedBranchIDs(ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()))
	obj.SetQueuedTime(time.Now())
	obj.SetScheduled(true)
	obj.SetBooked(true)

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.MessageID)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}
