package blockfactory

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/mockedvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const tscThreshold = time.Minute

func TestReferenceProvider_References1(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := protocol.NewTestFramework(t, workers.CreateGroup("Protocol"), new(mockedvm.MockedVM))
	tf.Instance.Run()

	tf.Engine.VirtualVoting.CreateIdentity("V1", 10)
	tf.Engine.VirtualVoting.CreateIdentity("V2", 20)

	tf.Engine.BlockDAG.CreateBlock("Block1", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX1", 3, "Genesis")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V1").PublicKey()))
	tf.Engine.BlockDAG.CreateBlock("Block2", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX2", 1, "TX1.0")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V1").PublicKey()))
	tf.Engine.BlockDAG.CreateBlock("Block3", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX3", 1, "TX1.1")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V1").PublicKey()))
	tf.Engine.BlockDAG.CreateBlock("Block4", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX4", 1, "TX1.0", "TX1.1")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V2").PublicKey()))
	tf.Engine.BlockDAG.IssueBlocks("Block1", "Block2", "Block3", "Block4")

	workers.WaitChildren()

	rp := NewReferenceProvider(tf.Instance, 30*time.Second, func() slot.Index {
		return 0
	})

	checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
		models.StrongParentType:      tf.Engine.BlockDAG.BlockIDs("Block3", "Block4"),
		models.ShallowLikeParentType: tf.Engine.BlockDAG.BlockIDs("Block4"),
	})
}

func TestBlockFactory_PrepareLikedReferences_2(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := protocol.NewTestFramework(t, workers.CreateGroup("Protocol"), new(mockedvm.MockedVM), protocol.WithProtocolOptions(protocol.WithTipManagerOptions(tipmanager.WithTimeSinceConfirmationThreshold(tscThreshold))))
	tf.Instance.Run()

	tf.Engine.VirtualVoting.CreateIdentity("V1", 10)
	tf.Engine.VirtualVoting.CreateIdentity("V2", 20)

	tf.Engine.BlockDAG.CreateBlock("Block0", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX0", 2, "Genesis")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V1").PublicKey()))
	tf.Engine.BlockDAG.CreateBlock("Block1", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX1", 1, "TX0.0")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V2").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute)))
	tf.Engine.BlockDAG.CreateBlock("Block2", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX2", 1, "TX0.1")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V2").PublicKey()))
	tf.Engine.BlockDAG.CreateBlock("Block3", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX3", 1, "TX0.1")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V1").PublicKey()))
	tf.Engine.BlockDAG.CreateBlock("Block4", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX4", 1, "TX0.0")), models.WithIssuer(tf.Engine.VirtualVoting.Identity("V1").PublicKey()))
	tf.Engine.BlockDAG.IssueBlocks("Block0", "Block1", "Block2", "Block3", "Block4")
	workers.WaitChildren()

	rp := NewReferenceProvider(tf.Instance, tscThreshold, func() slot.Index {
		return 0
	})

	// Verify that like references are set correctly.
	{
		checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs(), map[models.ParentsType]models.BlockIDs{}, true)

		checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs("Block2", "Block3"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Engine.BlockDAG.BlockIDs("Block2", "Block3"),
			models.ShallowLikeParentType: tf.Engine.BlockDAG.BlockIDs("Block2"),
		})

		checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs("Block1", "Block2"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Engine.BlockDAG.BlockIDs("Block1", "Block2"),
		})

		checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Engine.BlockDAG.BlockIDs("Block3", "Block4"),
			models.ShallowLikeParentType: tf.Engine.BlockDAG.BlockIDs("Block1", "Block2"),
		})

		checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs("Block1", "Block2", "Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Engine.BlockDAG.BlockIDs("Block1", "Block2", "Block3", "Block4"),
			models.ShallowLikeParentType: tf.Engine.BlockDAG.BlockIDs("Block1", "Block2"),
		})
	}

	// Add reattachment that is older than the original block and verify that it is selected with a like reference.
	{
		tf.Engine.BlockDAG.CreateBlock("Block5", models.WithPayload(tf.Engine.MemPool.Transaction("TX1")))
		tf.Engine.BlockDAG.IssueBlocks("Block5")
		workers.WaitChildren()

		checkReferences(t, rp, nil, tf.Engine.BlockDAG.BlockIDs("Block3", "Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType:      tf.Engine.BlockDAG.BlockIDs("Block3", "Block4"),
			models.ShallowLikeParentType: tf.Engine.BlockDAG.BlockIDs("Block2", "Block1"),
		})
	}
}

// Tests if weak references are properly constructed from consumed outputs.
func TestBlockFactory_WeakReferencesConsumed(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := protocol.NewTestFramework(t, workers.CreateGroup("Protocol"), new(mockedvm.MockedVM), protocol.WithProtocolOptions(protocol.WithTipManagerOptions(tipmanager.WithTimeSinceConfirmationThreshold(tscThreshold))))
	tf.Instance.Run()

	tf.Engine.BlockDAG.CreateBlock("Block1", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX1", 3, "Genesis")))
	tf.Engine.BlockDAG.CreateBlock("Block2", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX2", 1, "TX1.0")))
	tf.Engine.BlockDAG.CreateBlock("Block3", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX3", 1, "TX1.1")))
	tf.Engine.BlockDAG.CreateBlock("Block4", models.WithPayload(tf.Engine.MemPool.CreateTransaction("TX4", 1, "TX2.0", "TX3.0")))
	tf.Engine.BlockDAG.IssueBlocks("Block1", "Block2", "Block3", "Block4")
	workers.WaitChildren()

	rp := NewReferenceProvider(tf.Instance, tscThreshold, func() slot.Index {
		return 0
	})

	{
		checkReferences(t, rp, tf.Engine.BlockDAG.Block("Block2").Payload(), tf.Engine.BlockDAG.BlockIDs("Block1"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Engine.BlockDAG.BlockIDs("Block1"),
		})

		checkReferences(t, rp, tf.Engine.BlockDAG.Block("Block3").Payload(), tf.Engine.BlockDAG.BlockIDs("Block2"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Engine.BlockDAG.BlockIDs("Block2"),
			models.WeakParentType:   tf.Engine.BlockDAG.BlockIDs("Block1"),
		})

		checkReferences(t, rp, tf.Engine.BlockDAG.Block("Block4").Payload(), tf.Engine.BlockDAG.BlockIDs("Block4"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Engine.BlockDAG.BlockIDs("Block4"),
			models.WeakParentType:   tf.Engine.BlockDAG.BlockIDs("Block2", "Block3"),
		})

		checkReferences(t, rp, tf.Engine.BlockDAG.Block("Block4").Payload(), tf.Engine.BlockDAG.BlockIDs("Block4", "Block3"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Engine.BlockDAG.BlockIDs("Block4", "Block3"),
			models.WeakParentType:   tf.Engine.BlockDAG.BlockIDs("Block2"),
		})
	}

	// IssueBlock reattachment of TX3 (Block5) and make sure it is referenced in favor of Block3 (earliest attachment).
	tf.Engine.BlockDAG.CreateBlock("Block5", models.WithPayload(tf.Engine.MemPool.Transaction("TX3")))
	tf.Engine.BlockDAG.IssueBlocks("Block5")
	workers.WaitChildren()

	{
		checkReferences(t, rp, tf.Engine.BlockDAG.Block("Block4").Payload(), tf.Engine.BlockDAG.BlockIDs("Block1"), map[models.ParentsType]models.BlockIDs{
			models.StrongParentType: tf.Engine.BlockDAG.BlockIDs("Block1"),
			models.WeakParentType:   tf.Engine.BlockDAG.BlockIDs("Block5", "Block2"),
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
