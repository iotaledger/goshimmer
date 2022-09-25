package blockfactory

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

func TestReferenceProvider_References1(t *testing.T) {
	tf := engine.NewTestFramework(t)

	tf.Tangle.CreateIdentity("V1", validator.WithWeight(10))
	tf.Tangle.CreateIdentity("V2", validator.WithWeight(20))

	tf.Tangle.CreateBlock("Block1", models.WithPayload(tf.Tangle.CreateTransaction("TX1", 3, "Genesis")), models.WithIssuer(tf.Tangle.Identity("V1").PublicKey()))
	tf.Tangle.CreateBlock("Block2", models.WithPayload(tf.Tangle.CreateTransaction("TX2", 1, "TX1.0")), models.WithIssuer(tf.Tangle.Identity("V1").PublicKey()))
	tf.Tangle.CreateBlock("Block3", models.WithPayload(tf.Tangle.CreateTransaction("TX3", 1, "TX1.1")), models.WithIssuer(tf.Tangle.Identity("V1").PublicKey()))
	tf.Tangle.CreateBlock("Block4", models.WithPayload(tf.Tangle.CreateTransaction("TX4", 1, "TX1.0", "TX1.1")), models.WithIssuer(tf.Tangle.Identity("V2").PublicKey()))
	tf.Tangle.IssueBlocks("Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(func() *engine.Engine { return tf.Engine }, func() (epoch.Index, error) {
		return 0, nil
	})

	checkReferences(t, rp, nil, tf.Tangle.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
		models.StrongParentType:      tf.Tangle.BlockIDs("Block3", "Block4"),
		models.ShallowLikeParentType: tf.Tangle.BlockIDs("Block4"),
	})
}

func TestBlockFactory_PrepareLikedReferences_2(t *testing.T) {
	tf := engine.NewTestFramework(t)

	tf.Tangle.CreateIdentity("V1", validator.WithWeight(10))
	tf.Tangle.CreateIdentity("V2", validator.WithWeight(20))

	tf.Tangle.CreateBlock("Block0", models.WithPayload(tf.Tangle.CreateTransaction("TX0", 2, "Genesis")), models.WithIssuer(tf.Tangle.Identity("V1").PublicKey()))
	tf.Tangle.CreateBlock("Block1", models.WithPayload(tf.Tangle.CreateTransaction("TX1", 1, "TX0.0")), models.WithIssuer(tf.Tangle.Identity("V2").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute)))
	tf.Tangle.CreateBlock("Block2", models.WithPayload(tf.Tangle.CreateTransaction("TX2", 1, "TX0.1")), models.WithIssuer(tf.Tangle.Identity("V2").PublicKey()))
	tf.Tangle.CreateBlock("Block3", models.WithPayload(tf.Tangle.CreateTransaction("TX3", 1, "TX0.1")), models.WithIssuer(tf.Tangle.Identity("V1").PublicKey()))
	tf.Tangle.CreateBlock("Block4", models.WithPayload(tf.Tangle.CreateTransaction("TX4", 1, "TX0.0")), models.WithIssuer(tf.Tangle.Identity("V1").PublicKey()))
	tf.Tangle.IssueBlocks("Block0", "Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(func() *engine.Engine { return tf.Engine }, func() (epoch.Index, error) {
		return 0, nil
	})

	// Verify that like references are set correctly.
	{
		checkReferences(t, rp, nil, tf.Tangle.BlockIDs(), map[models.ParentsType]models.BlockIDs{}, true)

		checkReferences(t, rp, nil, tf.Tangle.BlockIDs("Block2", "Block3"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Tangle.BlockIDs("Block2", "Block3"),
			models.ShallowLikeParentType: tf.Tangle.BlockIDs("Block2"),
		})

		checkReferences(t, rp, nil, tf.Tangle.BlockIDs("Block1", "Block2"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Tangle.BlockIDs("Block1", "Block2"),
		})

		checkReferences(t, rp, nil, tf.Tangle.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Tangle.BlockIDs("Block3", "Block4"),
			models.ShallowLikeParentType: tf.Tangle.BlockIDs("Block1", "Block2"),
		})

		checkReferences(t, rp, nil, tf.Tangle.BlockIDs("Block1", "Block2", "Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Tangle.BlockIDs("Block1", "Block2", "Block3", "Block4"),
			models.ShallowLikeParentType: tf.Tangle.BlockIDs("Block1", "Block2"),
		})
	}

	// Add reattachment that is older than the original block and verify that it is selected with a like reference.
	{
		tf.Tangle.CreateBlock("Block5", models.WithPayload(tf.Tangle.Transaction("TX1")))
		tf.Tangle.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		checkReferences(t, rp, nil, tf.Tangle.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Tangle.BlockIDs("Block3", "Block4"),
			models.ShallowLikeParentType: tf.Tangle.BlockIDs("Block2", "Block5"),
		})
	}
}

// Tests if weak references are properly constructed from consumed outputs.
func TestBlockFactory_WeakReferencesConsumed(t *testing.T) {
	tf := engine.NewTestFramework(t)

	tf.Tangle.CreateBlock("Block1", models.WithPayload(tf.Tangle.CreateTransaction("TX1", 3, "Genesis")))
	tf.Tangle.CreateBlock("Block2", models.WithPayload(tf.Tangle.CreateTransaction("TX2", 1, "TX1.0")))
	tf.Tangle.CreateBlock("Block3", models.WithPayload(tf.Tangle.CreateTransaction("TX3", 1, "TX1.1")))
	tf.Tangle.CreateBlock("Block4", models.WithPayload(tf.Tangle.CreateTransaction("TX4", 1, "TX2.0", "TX3.0")))
	tf.Tangle.IssueBlocks("Block1", "Block2", "Block3", "Block4").WaitUntilAllTasksProcessed()

	rp := NewReferenceProvider(func() *engine.Engine { return tf.Engine }, func() (epoch.Index, error) {
		return 0, nil
	})

	{
		checkReferences(t, rp, tf.Tangle.Block("Block2").Payload(), tf.Tangle.BlockIDs("Block1"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Tangle.BlockIDs("Block1"),
		})

		checkReferences(t, rp, tf.Tangle.Block("Block3").Payload(), tf.Tangle.BlockIDs("Block2"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Tangle.BlockIDs("Block2"),
			models.WeakParentType:   tf.Tangle.BlockIDs("Block1"),
		})

		checkReferences(t, rp, tf.Tangle.Block("Block4").Payload(), tf.Tangle.BlockIDs("Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Tangle.BlockIDs("Block4"),
			models.WeakParentType:   tf.Tangle.BlockIDs("Block2", "Block3"),
		})

		checkReferences(t, rp, tf.Tangle.Block("Block4").Payload(), tf.Tangle.BlockIDs("Block4", "Block3"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Tangle.BlockIDs("Block4", "Block3"),
			models.WeakParentType:   tf.Tangle.BlockIDs("Block2"),
		})
	}

	// IssueBlock reattachment of TX3 (Block5) and make sure it is referenced in favor of Block3 (earliest attachment).
	tf.Tangle.CreateBlock("Block5", models.WithPayload(tf.Tangle.Transaction("TX3")))
	tf.Tangle.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

	{
		checkReferences(t, rp, tf.Tangle.Block("Block4").Payload(), tf.Tangle.BlockIDs("Block1"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Tangle.BlockIDs("Block1"),
			models.WeakParentType:   tf.Tangle.BlockIDs("Block5", "Block2"),
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
