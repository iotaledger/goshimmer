package tangle

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	fmt.Println("Bytes", len(msg.BytesOld()), msg.BytesOld())
	fmt.Println("Serix", len(msg.Bytes()), msg.Bytes())

	assert.Equal(t, msg.ObjectStorageKeyOld(), msg.ObjectStorageKey())
	assert.Equal(t, msg.ObjectStorageValueOld(), msg.ObjectStorageValue())

	objRestored, err := new(Message).FromObjectStorage(msg.ObjectStorageKey(), msg.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, msg.Bytes(), objRestored.(*Message).Bytes())
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

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(MarkerMessageMapping).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*MarkerMessageMapping).Bytes())
}

func TestSerixBranchWeight(t *testing.T) {
	obj := NewBranchWeight(ledgerstate.BranchIDFromRandomness())
	obj.SetWeight(0.65)

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(BranchWeight).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*BranchWeight).Bytes())
}

func TestSerixLatestMarkerVotes(t *testing.T) {
	obj := NewLatestMarkerVotes(1, identity.GenerateLocalIdentity().ID())

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(LatestMarkerVotes).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*LatestMarkerVotes).Bytes())
}

func TestSerixLatestBranchVotes(t *testing.T) {
	obj := NewLatestBranchVotes(identity.GenerateLocalIdentity().ID())
	obj.Store(new(BranchVote).WithBranchID(ledgerstate.BranchIDFromRandomness()).WithOpinion(Confirmed))

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(LatestBranchVotes).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*LatestBranchVotes).Bytes())
}

func TestSerixBranchVoters(t *testing.T) {
	obj := NewBranchVoters(ledgerstate.BranchIDFromRandomness())
	voters := NewVoters()
	voters.SerializableSet.Add(identity.GenerateLocalIdentity().ID())
	voters.SerializableSet.Add(identity.GenerateLocalIdentity().ID())
	obj.AddVoters(voters)

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(BranchVoters).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*BranchVoters).Bytes())
}

func TestSerixMarkerIndexBranchIDMapping(t *testing.T) {
	obj := NewMarkerIndexBranchIDMapping(1)
	obj.SetBranchIDs(3, ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()))

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(MarkerIndexBranchIDMapping).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*MarkerIndexBranchIDMapping).Bytes())
}

func TestSerixAttachment(t *testing.T) {
	obj := NewAttachment(randomTransaction().ID(), randomMessageID())

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())

	objRestored, err := new(Attachment).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*Attachment).Bytes())
}

func TestSerixMissingMessage(t *testing.T) {
	obj := NewMissingMessage(randomMessageID())

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(MissingMessage).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*MissingMessage).Bytes())
}

func TestSerixApprover(t *testing.T) {
	obj := NewApprover(StrongApprover, randomMessageID(), randomMessageID())

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())

	objRestored, err := new(Approver).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*Approver).Bytes())
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

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(MessageMetadata).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*MessageMetadata).Bytes())
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

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(MessageMetadata).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	// TODO: err here - restored object contains non-nil structure details when should be nil
	assert.Equal(t, obj.Bytes(), objRestored.(*MessageMetadata).Bytes())
}
