package blockfilter

import (
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestFramework struct {
	Test             *testing.T
	SlotTimeProvider *slot.TimeProvider
	Filter           *Filter
}

func NewTestFramework(t *testing.T, slotTimeProvider *slot.TimeProvider, optsFilter ...options.Option[Filter]) *TestFramework {
	tf := &TestFramework{
		Test:             t,
		SlotTimeProvider: slotTimeProvider,
		Filter:           New(optsFilter...),
	}

	tf.Filter.Events().BlockAllowed.Hook(func(block *models.Block) {
		t.Logf("BlockAllowed: %s", block.ID())
	})

	tf.Filter.Events().BlockFiltered.Hook(func(event *filter.BlockFilteredEvent) {
		t.Logf("BlockFiltered: %s - %s", event.Block.ID(), event.Reason)
	})

	return tf
}

func (t *TestFramework) processBlock(alias string, block *models.Block) {
	require.NoError(t.Test, block.DetermineID(t.SlotTimeProvider))
	block.ID().RegisterAlias(alias)
	t.Filter.ProcessReceivedBlock(block, identity.NewID(ed25519.PublicKey{}))
}

func (t *TestFramework) IssueUnsignedBlockAtTime(alias string, issuingTime time.Time) {
	block := models.NewBlock(
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
		models.WithIssuingTime(issuingTime),
	)
	t.processBlock(alias, block)
}

func (t *TestFramework) IssueUnsignedBlockAtSlot(alias string, index slot.Index, committing slot.Index) {
	block := models.NewBlock(
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
		models.WithIssuingTime(t.SlotTimeProvider.StartTime(index)),
		models.WithCommitment(commitment.New(committing, commitment.ID{}, types.Identifier{}, 0)),
	)
	t.processBlock(alias, block)
}

func (t *TestFramework) IssueSigned(alias string) {
	block := models.NewBlock(
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
		models.WithIssuingTime(time.Now()),
	)
	keyPair := ed25519.GenerateKeyPair()
	require.NoError(t.Test, block.Sign(&keyPair))

	t.processBlock(alias, block)
}

func TestFilter_WithMaxAllowedWallClockDrift(t *testing.T) {
	allowedDrift := 3 * time.Second

	tf := NewTestFramework(t,
		slot.NewTimeProvider(time.Now().Unix(), 10),
		WithMaxAllowedWallClockDrift(allowedDrift),
		WithSignatureValidation(false),
	)

	tf.Filter.Events().BlockAllowed.Hook(func(block *models.Block) {
		require.NotEqual(t, "tooFarAheadFuture", block.ID().Alias())
	})

	tf.Filter.Events().BlockFiltered.Hook(func(event *filter.BlockFilteredEvent) {
		require.Equal(t, "tooFarAheadFuture", event.Block.ID().Alias())
		require.True(t, errors.Is(event.Reason, ErrorsBlockTimeTooFarAheadInFuture))
	})

	tf.IssueUnsignedBlockAtTime("past", time.Now().Add(-allowedDrift))
	tf.IssueUnsignedBlockAtTime("present", time.Now())
	tf.IssueUnsignedBlockAtTime("acceptedFuture", time.Now().Add(allowedDrift))
	tf.IssueUnsignedBlockAtTime("tooFarAheadFuture", time.Now().Add(allowedDrift).Add(1*time.Second))
}

func TestFilter_WithSignatureValidation(t *testing.T) {
	tf := NewTestFramework(t,
		slot.NewTimeProvider(time.Now().Unix(), 10),
		WithSignatureValidation(true),
	)

	tf.Filter.Events().BlockAllowed.Hook(func(block *models.Block) {
		require.Equal(t, "valid", block.ID().Alias())
	})

	tf.Filter.Events().BlockFiltered.Hook(func(event *filter.BlockFilteredEvent) {
		require.Equal(t, "invalid", event.Block.ID().Alias())
		require.True(t, errors.Is(event.Reason, ErrorsInvalidSignature))
	})

	tf.IssueUnsignedBlockAtTime("invalid", time.Now())
	tf.IssueSigned("valid")
}

func TestFilter_MinCommittableSlotAge(t *testing.T) {
	tf := NewTestFramework(t,
		slot.NewTimeProvider(time.Now().Add(-5*time.Minute).Unix(), 10),
		WithMinCommittableSlotAge(3),
		WithSignatureValidation(false),
	)

	tf.Filter.Events().BlockAllowed.Hook(func(block *models.Block) {
		require.True(t, strings.HasPrefix(block.ID().Alias(), "valid"))
	})

	tf.Filter.Events().BlockFiltered.Hook(func(event *filter.BlockFilteredEvent) {
		require.True(t, strings.HasPrefix(event.Block.ID().Alias(), "invalid"))
		require.True(t, errors.Is(event.Reason, ErrorCommitmentNotCommittable))
	})

	tf.IssueUnsignedBlockAtSlot("valid-1-0", 1, 0)
	tf.IssueUnsignedBlockAtSlot("valid-2-0", 2, 0)
	tf.IssueUnsignedBlockAtSlot("valid-3-0", 3, 0)
	tf.IssueUnsignedBlockAtSlot("valid-4-0", 4, 0)

	tf.IssueUnsignedBlockAtSlot("invalid-1-1", 1, 1)
	tf.IssueUnsignedBlockAtSlot("invalid-2-1", 2, 1)
	tf.IssueUnsignedBlockAtSlot("invalid-3-1", 3, 1)
	tf.IssueUnsignedBlockAtSlot("valid-4-1", 4, 1)
	tf.IssueUnsignedBlockAtSlot("valid-5-1", 5, 1)

	tf.IssueUnsignedBlockAtSlot("valid-5-0", 5, 0)
	tf.IssueUnsignedBlockAtSlot("valid-4-1", 5, 1)
	tf.IssueUnsignedBlockAtSlot("valid-5-2", 5, 2)
	tf.IssueUnsignedBlockAtSlot("invalid-5-3", 5, 3)
	tf.IssueUnsignedBlockAtSlot("invalid-5-4", 5, 4)
	tf.IssueUnsignedBlockAtSlot("invalid-5-5", 5, 5)
	tf.IssueUnsignedBlockAtSlot("invalid-5-6", 5, 6)
}
