package blockfactory

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models/payload"
)

func TestFactory_IssuePayload(t *testing.T) {
	localIdentity := identity.GenerateLocalIdentity()

	ecRecord := epoch.NewECRecord(10)
	ecRecord.SetECR([32]byte{123, 255})
	ecRecord.SetPrevEC([32]byte{90, 111})
	confirmedEpochIndex := epoch.Index(25)
	commitmentFunc := func() (*epoch.ECRecord, epoch.Index, error) {
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
	createdBlock, err := factory.IssuePayload(pay, 2)
	require.NoError(t, err)

	assert.Contains(t, createdBlock.ParentsByType(models.StrongParentType), block1.ID(), block2.ID())
	assert.Equal(t, localIdentity.PublicKey(), createdBlock.IssuerPublicKey())
	// issuingTime
	assert.Equal(t, createdBlock.IssuingTime(), block1.IssuingTime().Add(time.Second))
	assert.EqualValues(t, 1337, createdBlock.SequenceNumber())
	assert.Equal(t, lo.PanicOnErr(pay.Bytes()), lo.PanicOnErr(createdBlock.Payload().Bytes()))
	assert.Equal(t, ecRecord.EI(), createdBlock.EI())
	assert.Equal(t, ecRecord.ECR(), createdBlock.ECR())
	assert.Equal(t, ecRecord.PrevEC(), createdBlock.PrevEC())
	assert.Equal(t, confirmedEpochIndex, createdBlock.LatestConfirmedEpoch())
	assert.EqualValues(t, 42, createdBlock.Nonce())

	signatureValid, err := createdBlock.VerifySignature()
	require.NoError(t, err)
	assert.True(t, signatureValid)
}
