package blockfactory

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

func TestReferenceProvider_References1(t *testing.T) {
	tf := protocol.NewTestFramework(t)
	tf.Protocol.Run()

	tangleTF := tangle.NewTestFramework(t, tangle.WithTangle(tf.Protocol.Engine().Tangle))

	tangleTF.CreateIdentity("V1", validator.WithWeight(10))
	tangleTF.CreateIdentity("V2", validator.WithWeight(20))

	tangleTF.CreateBlock("Block1", models.WithPayload(tangleTF.CreateTransaction("TX1", 3, "Genesis")), models.WithIssuer(tangleTF.Identity("V1").PublicKey()))
	tangleTF.CreateBlock("Block2", models.WithPayload(tangleTF.CreateTransaction("TX2", 1, "TX1.0")), models.WithIssuer(tangleTF.Identity("V1").PublicKey()))
	tangleTF.CreateBlock("Block3", models.WithPayload(tangleTF.CreateTransaction("TX3", 1, "TX1.1")), models.WithIssuer(tangleTF.Identity("V1").PublicKey()))
	tangleTF.CreateBlock("Block4", models.WithPayload(tangleTF.CreateTransaction("TX4", 1, "TX1.0", "TX1.1")), models.WithIssuer(tangleTF.Identity("V2").PublicKey()))
	tangleTF.IssueBlocks("Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(tf.Protocol.Engine, func() epoch.Index {
		return 0
	})

	checkReferences(t, rp, nil, tangleTF.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
		models.StrongParentType:      tangleTF.BlockIDs("Block3", "Block4"),
		models.ShallowLikeParentType: tangleTF.BlockIDs("Block4"),
	})
}

func TestBlockFactory_PrepareLikedReferences_2(t *testing.T) {
	tf := protocol.NewTestFramework(t)
	tf.Protocol.Run()

	tangleTF := tangle.NewTestFramework(t, tangle.WithTangle(tf.Protocol.Engine().Tangle))

	tangleTF.CreateIdentity("V1", validator.WithWeight(10))
	tangleTF.CreateIdentity("V2", validator.WithWeight(20))

	tangleTF.CreateBlock("Block0", models.WithPayload(tangleTF.CreateTransaction("TX0", 2, "Genesis")), models.WithIssuer(tangleTF.Identity("V1").PublicKey()))
	tangleTF.CreateBlock("Block1", models.WithPayload(tangleTF.CreateTransaction("TX1", 1, "TX0.0")), models.WithIssuer(tangleTF.Identity("V2").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute)))
	tangleTF.CreateBlock("Block2", models.WithPayload(tangleTF.CreateTransaction("TX2", 1, "TX0.1")), models.WithIssuer(tangleTF.Identity("V2").PublicKey()))
	tangleTF.CreateBlock("Block3", models.WithPayload(tangleTF.CreateTransaction("TX3", 1, "TX0.1")), models.WithIssuer(tangleTF.Identity("V1").PublicKey()))
	tangleTF.CreateBlock("Block4", models.WithPayload(tangleTF.CreateTransaction("TX4", 1, "TX0.0")), models.WithIssuer(tangleTF.Identity("V1").PublicKey()))
	tangleTF.IssueBlocks("Block0", "Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(tf.Protocol.Engine, func() epoch.Index {
		return 0
	})

	// Verify that like references are set correctly.
	{
		checkReferences(t, rp, nil, tangleTF.BlockIDs(), map[models.ParentsType]models.BlockIDs{}, true)

		checkReferences(t, rp, nil, tangleTF.BlockIDs("Block2", "Block3"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tangleTF.BlockIDs("Block2", "Block3"),
			models.ShallowLikeParentType: tangleTF.BlockIDs("Block2"),
		})

		checkReferences(t, rp, nil, tangleTF.BlockIDs("Block1", "Block2"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tangleTF.BlockIDs("Block1", "Block2"),
		})

		checkReferences(t, rp, nil, tangleTF.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tangleTF.BlockIDs("Block3", "Block4"),
			models.ShallowLikeParentType: tangleTF.BlockIDs("Block1", "Block2"),
		})

		checkReferences(t, rp, nil, tangleTF.BlockIDs("Block1", "Block2", "Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tangleTF.BlockIDs("Block1", "Block2", "Block3", "Block4"),
			models.ShallowLikeParentType: tangleTF.BlockIDs("Block1", "Block2"),
		})
	}

	// Add reattachment that is older than the original block and verify that it is selected with a like reference.
	{
		tangleTF.CreateBlock("Block5", models.WithPayload(tangleTF.Transaction("TX1")))
		tangleTF.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		checkReferences(t, rp, nil, tangleTF.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tangleTF.BlockIDs("Block3", "Block4"),
			models.ShallowLikeParentType: tangleTF.BlockIDs("Block2", "Block5"),
		})
	}
}

// Tests if weak references are properly constructed from consumed outputs.
func TestBlockFactory_WeakReferencesConsumed(t *testing.T) {
	tf := protocol.NewTestFramework(t)
	tf.Protocol.Run()

	tangleTF := tangle.NewTestFramework(t, tangle.WithTangle(tf.Protocol.Engine().Tangle))

	tangleTF.CreateBlock("Block1", models.WithPayload(tangleTF.CreateTransaction("TX1", 3, "Genesis")))
	tangleTF.CreateBlock("Block2", models.WithPayload(tangleTF.CreateTransaction("TX2", 1, "TX1.0")))
	tangleTF.CreateBlock("Block3", models.WithPayload(tangleTF.CreateTransaction("TX3", 1, "TX1.1")))
	tangleTF.CreateBlock("Block4", models.WithPayload(tangleTF.CreateTransaction("TX4", 1, "TX2.0", "TX3.0")))
	tangleTF.IssueBlocks("Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(tf.Protocol.Engine, func() epoch.Index {
		return 0
	})

	{
		checkReferences(t, rp, tangleTF.Block("Block2").Payload(), tangleTF.BlockIDs("Block1"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tangleTF.BlockIDs("Block1"),
		})

		checkReferences(t, rp, tangleTF.Block("Block3").Payload(), tangleTF.BlockIDs("Block2"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tangleTF.BlockIDs("Block2"),
			models.WeakParentType:   tangleTF.BlockIDs("Block1"),
		})

		checkReferences(t, rp, tangleTF.Block("Block4").Payload(), tangleTF.BlockIDs("Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tangleTF.BlockIDs("Block4"),
			models.WeakParentType:   tangleTF.BlockIDs("Block2", "Block3"),
		})

		checkReferences(t, rp, tangleTF.Block("Block4").Payload(), tangleTF.BlockIDs("Block4", "Block3"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tangleTF.BlockIDs("Block4", "Block3"),
			models.WeakParentType:   tangleTF.BlockIDs("Block2"),
		})
	}

	// IssueBlock reattachment of TX3 (Block5) and make sure it is referenced in favor of Block3 (earliest attachment).
	tangleTF.CreateBlock("Block5", models.WithPayload(tangleTF.Transaction("TX3")))
	tangleTF.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

	{
		checkReferences(t, rp, tangleTF.Block("Block4").Payload(), tangleTF.BlockIDs("Block1"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tangleTF.BlockIDs("Block1"),
			models.WeakParentType:   tangleTF.BlockIDs("Block5", "Block2"),
		})
	}
}

func checkReferences(t *testing.T, rp *ReferenceProvider, p payload.Payload, parents models.BlockIDs, expectedReferences map[models.ParentsType]models.BlockIDs, errorExpected ...bool) {
	actualReferences, err := rp.References(p, parents)
	if len(errorExpected) > 0 && errorExpected[0] {
		fmt.Println(err)
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	for _, referenceType := range []models.ParentsType{models.StrongParentType, models.ShallowLikeParentType, models.WeakParentType} {
		assert.Equalf(t, expectedReferences[referenceType], actualReferences[referenceType], "references type %s do not match: expected %s - actual %s", referenceType, expectedReferences[referenceType], actualReferences[referenceType])
	}
}
