package blockfactory

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

func TestFactory_IssuePayload(t *testing.T) {
	localIdentity := identity.GenerateLocalIdentity()

	ecRecord := commitment.New(1, commitment.NewID(1, []byte{90, 111}), types.NewIdentifier([]byte{123, 255}), 1)
	confirmedEpochIndex := epoch.Index(25)
	commitmentFunc := func() (*commitment.Commitment, epoch.Index, error) {
		return ecRecord, confirmedEpochIndex, nil
	}

	referencesFunc := func(payload payload.Payload, strongParents models.BlockIDs) (references models.ParentBlockIDs, err error) {
		return models.NewParentBlockIDs().AddAll(models.StrongParentType, strongParents), nil
	}

	block1 := blockdag.NewBlock(models.NewBlock(
		models.WithIssuingTime(time.Now().Add(+5 * time.Minute)),
	))
	block2 := blockdag.NewBlock(models.NewBlock(
		models.WithIssuingTime(time.Now().Add(-5 * time.Minute)),
	))
	blockRetriever := func(blockID models.BlockID) (block *blockdag.Block, exists bool) {
		if blockID == block1.ID() {
			return block1, true
		}
		if blockID == block2.ID() {
			return block2, true
		}

		return nil, false
	}

	tipSelectorFunc := func(countParents int) models.BlockIDs {
		return models.NewBlockIDs(block1.ID(), block2.ID())
	}

	pay := payload.NewGenericDataPayload([]byte("test"))

	factory := NewBlockFactory(localIdentity, blockRetriever, tipSelectorFunc, referencesFunc, commitmentFunc)
	createdBlock, err := factory.CreateBlock(pay, 2)
	require.NoError(t, err)

	assert.Contains(t, createdBlock.ParentsByType(models.StrongParentType), block1.ID(), block2.ID())
	assert.Equal(t, localIdentity.PublicKey(), createdBlock.IssuerPublicKey())
	// issuingTime
	assert.Equal(t, createdBlock.IssuingTime(), block1.IssuingTime().Add(time.Second))
	assert.EqualValues(t, 0, createdBlock.SequenceNumber())
	assert.Equal(t, lo.PanicOnErr(pay.Bytes()), lo.PanicOnErr(createdBlock.Payload().Bytes()))
	assert.Equal(t, ecRecord.Index(), createdBlock.Commitment().Index())
	assert.Equal(t, ecRecord.RootsID(), createdBlock.Commitment().RootsID())
	assert.Equal(t, ecRecord.PrevID(), createdBlock.Commitment().PrevID())
	assert.Equal(t, confirmedEpochIndex, createdBlock.LatestConfirmedEpoch())
	assert.EqualValues(t, 0, createdBlock.Nonce())

	signatureValid, err := createdBlock.VerifySignature()
	require.NoError(t, err)
	assert.True(t, signatureValid)

	b := lo.PanicOnErr(createdBlock.Bytes())
	deserializedBlock := new(models.Block)
	if _, err = deserializedBlock.FromBytes(b); err != nil {
		panic(err)
	}
	require.Equal(t, b, lo.PanicOnErr(deserializedBlock.Bytes()))

	require.NoError(t, deserializedBlock.DetermineID())
	require.Equal(t, createdBlock.ID(), deserializedBlock.ID())

	signatureValid, err = deserializedBlock.VerifySignature()
	require.NoError(t, err)
	require.True(t, signatureValid)
}
