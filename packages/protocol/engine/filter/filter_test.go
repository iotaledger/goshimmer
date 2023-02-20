package filter

import (
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
)

type TestFramework struct {
	Test   *testing.T
	Filter *Filter
}

func NewTestFramework(t *testing.T, optsFilter ...options.Option[Filter]) *TestFramework {
	tf := &TestFramework{
		Test:   t,
		Filter: New(optsFilter...),
	}

	event.Hook(tf.Filter.Events.BlockAllowed, func(block *models.Block) {
		t.Logf("BlockAllowed: %s", block.ID())
	})

	event.Hook(tf.Filter.Events.BlockFiltered, func(event *BlockFilteredEvent) {
		t.Logf("BlockFiltered: %s - %s", event.Block.ID(), event.Reason)
	})

	return tf
}

func (t *TestFramework) processBlock(alias string, block *models.Block) {
	require.NoError(t.Test, block.DetermineID())
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

func (t *TestFramework) IssueUnsignedBlockAtEpoch(alias string, index epoch.Index, committing epoch.Index) {
	block := models.NewBlock(
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
		models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(index-1)*epoch.Duration, 0)),
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
		WithMaxAllowedWallClockDrift(allowedDrift),
		WithSignatureValidation(false),
	)

	event.Hook(tf.Filter.Events.BlockAllowed, func(block *models.Block) {
		require.NotEqual(t, "tooFarAheadFuture", block.ID().Alias())
	})

	event.Hook(tf.Filter.Events.BlockFiltered, func(event *BlockFilteredEvent) {
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
		WithSignatureValidation(true),
	)

	event.Hook(tf.Filter.Events.BlockAllowed, func(block *models.Block) {
		require.Equal(t, "valid", block.ID().Alias())
	})

	event.Hook(tf.Filter.Events.BlockFiltered, func(event *BlockFilteredEvent) {
		require.Equal(t, "invalid", event.Block.ID().Alias())
		require.True(t, errors.Is(event.Reason, ErrorsInvalidSignature))
	})

	tf.IssueUnsignedBlockAtTime("invalid", time.Now())
	tf.IssueSigned("valid")
}

func TestFilter_MinCommittableEpochAge(t *testing.T) {
	tf := NewTestFramework(t,
		WithMinCommittableEpochAge(30*time.Second), // 3 epochs
		WithSignatureValidation(false),
	)

	event.Hook(tf.Filter.Events.BlockAllowed, func(block *models.Block) {
		require.True(t, strings.HasPrefix(block.ID().Alias(), "valid"))
	})

	event.Hook(tf.Filter.Events.BlockFiltered, func(event *BlockFilteredEvent) {
		require.True(t, strings.HasPrefix(event.Block.ID().Alias(), "invalid"))
		require.True(t, errors.Is(event.Reason, ErrorCommitmentNotCommittable))
	})

	tf.IssueUnsignedBlockAtEpoch("valid-5", 5, 0)
	tf.IssueUnsignedBlockAtEpoch("valid-4", 5, 1)
	tf.IssueUnsignedBlockAtEpoch("valid-3", 5, 2)
	tf.IssueUnsignedBlockAtEpoch("invalid-2", 5, 3)
	tf.IssueUnsignedBlockAtEpoch("invalid-1", 5, 4)
	tf.IssueUnsignedBlockAtEpoch("invalid-0", 5, 5)
	tf.IssueUnsignedBlockAtEpoch("invalid+1", 5, 6)
}
