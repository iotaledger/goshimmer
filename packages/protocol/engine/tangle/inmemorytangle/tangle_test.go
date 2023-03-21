package inmemorytangle_test

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/testtangle"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func Test(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := testtangle.NewDefaultTestFramework(t,
		workers.CreateGroup("LedgerTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		slot.NewTimeProvider(time.Now().Unix(), 10),
	)

	tf.BlockDAG.CreateBlock("block1")
	tf.BlockDAG.CreateBlock("block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("block1")))
	tf.BlockDAG.IssueBlocks("block1", "block2")

	tf.BlockDAG.AssertSolid(map[string]bool{
		"block1": true,
		"block2": true,
	})
}
