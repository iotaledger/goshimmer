package tsc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/testtangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tsc"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestOrphanageManager_orphanBeforeTSC(t *testing.T) {
	threshold := 30 * time.Second

	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t,
		testtangle.NewDefaultTestFramework(t,
			workers.CreateGroup("TangleTestFramework"),
			realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
			slot.NewTimeProvider(time.Now().Unix(), 10),
		),
		tsc.WithTimeSinceConfirmationThreshold(threshold),
	)

	now := time.Now()
	for i := 0; i < 20; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		block := blockdag.NewBlock(tf.BlockDAG.CreateBlock(alias, models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(time.Duration(i)*time.Second))), blockdag.WithSolid(true))
		tf.Manager.AddBlock(booker.NewBlock(block))
	}

	tf.Manager.HandleTimeUpdate(now.Add(threshold).Add(10 * time.Second))
	tf.BlockDAG.AssertOrphanedCount(11, "%d blocks should be orphaned", 1)
}

func TestOrphanageManager_HandleTimeUpdate(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t,
		testtangle.NewDefaultTestFramework(t,
			workers.CreateGroup("TangleTestFramework"),
			realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
			slot.NewTimeProvider(time.Now().Add(-2*time.Hour).Unix(), 10),
		),
		tsc.WithTimeSinceConfirmationThreshold(30*time.Second),
	)

	createTestTangleOrphanage(tf)

	lo.MergeMaps(tf.MockAcceptance.AcceptedBlocks, models.BlockIDs{
		tf.BlockDAG.Block("Marker-0/1").ID():    types.Void,
		tf.BlockDAG.Block("0/1-preTSC_0").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_1").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_2").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_3").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_4").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_5").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_6").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_7").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_8").ID():  types.Void,
		tf.BlockDAG.Block("0/1-preTSC_9").ID():  types.Void,
		tf.BlockDAG.Block("0/1-postTSC_0").ID(): types.Void,
	})

	require.Equal(t, 27, tf.Manager.Size())

	for blockID := range tf.MockAcceptance.AcceptedBlocks {
		virtualVotingBlock, _ := tf.Booker.Instance.Block(blockID)
		tf.Manager.HandleTimeUpdate(virtualVotingBlock.IssuingTime())
	}

	workers.WaitChildren()

	tf.BlockDAG.AssertOrphanedCount(10, "expected %d orphaned blocks", 10)
	tf.AssertOrphaned(map[string]bool{
		"0/1-preTSCSeq1_0":  true,
		"0/1-preTSCSeq1_1":  true,
		"0/1-preTSCSeq1_2":  true,
		"0/1-preTSCSeq1_3":  true,
		"0/1-preTSCSeq1_4":  true,
		"0/1-preTSCSeq1_5":  true,
		"0/1-preTSCSeq1_6":  true,
		"0/1-preTSCSeq1_7":  true,
		"0/1-preTSCSeq1_8":  true,
		"0/1-preTSCSeq1_9":  true,
		"0/1-postTSCSeq1_0": false,
		"0/1-postTSCSeq1_1": false,
		"0/1-postTSCSeq1_2": false,
		"0/1-postTSCSeq1_3": false,
		"0/1-postTSCSeq1_4": false,
	})
	require.Equal(t, 6, tf.Manager.Size())

	// Mark orphaned blocks as accepted and make sure that they get unorphaned.
	{
		newAcceptedBlocksInOrder := []models.BlockID{
			tf.BlockDAG.Block("0/1-preTSCSeq1_0").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_1").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_2").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_3").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_4").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_5").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_6").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_7").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_8").ID(),
			tf.BlockDAG.Block("0/1-preTSCSeq1_9").ID(),
			tf.BlockDAG.Block("0/1-postTSCSeq1_0").ID(),
			tf.BlockDAG.Block("0/1-postTSCSeq1_1").ID(),
			tf.BlockDAG.Block("0/1-postTSCSeq1_2").ID(),
			tf.BlockDAG.Block("0/1-postTSCSeq1_3").ID(),
			tf.BlockDAG.Block("0/1-postTSCSeq1_4").ID(),
		}
		newAcceptedBlocks := models.BlockIDs{
			tf.BlockDAG.Block("0/1-preTSCSeq1_0").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_1").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_2").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_3").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_4").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_5").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_6").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_7").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_8").ID():  types.Void,
			tf.BlockDAG.Block("0/1-preTSCSeq1_9").ID():  types.Void,
			tf.BlockDAG.Block("0/1-postTSCSeq1_0").ID(): types.Void,
			tf.BlockDAG.Block("0/1-postTSCSeq1_1").ID(): types.Void,
			tf.BlockDAG.Block("0/1-postTSCSeq1_2").ID(): types.Void,
			tf.BlockDAG.Block("0/1-postTSCSeq1_3").ID(): types.Void,
			tf.BlockDAG.Block("0/1-postTSCSeq1_4").ID(): types.Void,
		}
		lo.MergeMaps(tf.MockAcceptance.AcceptedBlocks, newAcceptedBlocks)

		for _, blockID := range newAcceptedBlocksInOrder {
			virtualVotingBlock, _ := tf.Booker.Instance.Block(blockID)
			tf.Manager.HandleTimeUpdate(virtualVotingBlock.IssuingTime())
			tf.BlockDAG.Instance.SetOrphaned(virtualVotingBlock.Block, false)
		}

		workers.WaitChildren()

		tf.BlockDAG.AssertOrphanedCount(0, "expected %d orphaned blocks", 0)
		tf.AssertOrphaned(map[string]bool{
			"0/1-preTSCSeq1_0":  false,
			"0/1-preTSCSeq1_1":  false,
			"0/1-preTSCSeq1_2":  false,
			"0/1-preTSCSeq1_3":  false,
			"0/1-preTSCSeq1_4":  false,
			"0/1-preTSCSeq1_5":  false,
			"0/1-preTSCSeq1_6":  false,
			"0/1-preTSCSeq1_7":  false,
			"0/1-preTSCSeq1_8":  false,
			"0/1-preTSCSeq1_9":  false,
			"0/1-postTSCSeq1_0": false,
			"0/1-postTSCSeq1_1": false,
			"0/1-postTSCSeq1_2": false,
			"0/1-postTSCSeq1_3": false,
			"0/1-postTSCSeq1_4": false,
		})
		require.Equal(t, 6, tf.Manager.Size())
	}
}

func createTestTangleOrphanage(tf *TestFramework) {
	// SEQUENCE 0
	{
		tf.BlockDAG.CreateBlock("Marker-0/1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuingTime(time.Now().Add(-6*time.Minute)))
		tf.BlockDAG.IssueBlocks("Marker-0/1")
		lastMsgAlias := issueBlocks(tf, "0/1-preTSC", 10, []string{"Marker-0/1"}, time.Minute*6)
		issueBlocks(tf, "0/1-postTSC", 1, []string{lastMsgAlias}, 0)
	}

	// SEQUENCE 1
	{
		lastMsgAlias := issueBlocks(tf, "0/1-preTSCSeq1", 10, []string{"Marker-0/1"}, time.Minute*6)
		issueBlocks(tf, "0/1-postTSCSeq1", 5, []string{lastMsgAlias}, time.Second*15)
	}
}

func issueBlocks(tf *TestFramework, blkPrefix string, blkCount int, parents []string, timestampOffset time.Duration) string {
	blkAlias := fmt.Sprintf("%s_%d", blkPrefix, 0)

	tf.BlockDAG.CreateBlock(blkAlias, models.WithStrongParents(tf.BlockDAG.BlockIDs(parents...)), models.WithIssuingTime(time.Now().Add(-timestampOffset)))
	tf.BlockDAG.IssueBlocks(blkAlias)

	for i := 1; i < blkCount; i++ {
		alias := fmt.Sprintf("%s_%d", blkPrefix, i)
		tf.BlockDAG.CreateBlock(alias, models.WithIssuer(identity.GenerateIdentity().PublicKey()), models.WithStrongParents(tf.BlockDAG.BlockIDs(blkAlias)), models.WithIssuingTime(time.Now().Add(-timestampOffset)))
		tf.BlockDAG.IssueBlocks(alias)
		blkAlias = alias
	}
	return blkAlias
}
