package dashboard

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.NonBlockingQueuedWorkerPool

	blkHistoryMutex    sync.RWMutex
	blkFinalized       map[string]bool
	blkHistory         []*models.Block
	maxBlkHistorySize  = 1000
	numHistoryToRemove = 100
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string              `json:"id"`
	ParentIDsByType map[string][]string `json:"parentIDsByType"`
	IsFinalized     bool                `json:"is_finalized"`
	IsTx            bool                `json:"is_tx"`
}

// tipinfo holds information about whether a given block is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

// history holds a set of vertices in a DAG.
type history struct {
	Vertices []vertex `json:"vertices"`
}

func configureVisualizer() {
	visualizerWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *models.Block:
			sendVertex(x, task.Param(1).(bool))
		case *scheduler.Block:
			sendTipInfo(x, task.Param(1).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure blkHistory, blkSolid
	blkFinalized = make(map[string]bool, maxBlkHistorySize)
	blkHistory = make([]*models.Block, 0, maxBlkHistorySize)
}

func sendVertex(blk *models.Block, finalized bool) {
	broadcastWsBlock(&wsblk{MsgTypeVertex, &vertex{
		ID:              blk.ID().Base58(),
		ParentIDsByType: prepareParentReferences(blk),
		IsFinalized:     finalized,
		IsTx:            blk.Payload().Type() == devnetvm.TransactionType,
	}}, true)
}

func sendTipInfo(block *scheduler.Block, isTip bool) {
	broadcastWsBlock(&wsblk{MsgTypeTipInfo, &tipinfo{
		ID:    block.ID().Base58(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	processBlock := func(block *models.Block, accepted bool) {
		addToHistory(block, accepted)
		visualizerWorkerPool.TrySubmit(block, accepted)
	}

	notifyNewBlkStored := event.NewClosure(func(block *blockdag.Block) {
		processBlock(block.ModelsBlock, false)
	})

	notifyNewBlkAccepted := event.NewClosure(func(block *blockgadget.Block) {
		processBlock(block.ModelsBlock, block.IsAccepted())
	})

	notifyNewTip := event.NewClosure(func(block *scheduler.Block) {
		visualizerWorkerPool.TrySubmit(block, true)
	})

	notifyDeletedTip := event.NewClosure(func(block *scheduler.Block) {
		visualizerWorkerPool.TrySubmit(block, false)
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(notifyNewBlkStored)
		defer deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Detach(notifyNewBlkStored)
		deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(notifyNewBlkAccepted)
		defer deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Detach(notifyNewBlkAccepted)
		deps.Protocol.Events.TipManager.TipAdded.Attach(notifyNewTip)
		defer deps.Protocol.Events.TipManager.TipAdded.Detach(notifyNewTip)
		deps.Protocol.Events.TipManager.TipRemoved.Attach(notifyDeletedTip)
		defer deps.Protocol.Events.TipManager.TipRemoved.Detach(notifyDeletedTip)
		<-ctx.Done()
		log.Info("Stopping Dashboard[Visualizer] ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func setupVisualizerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/visualizer/history", func(c echo.Context) (err error) {
		blkHistoryMutex.RLock()
		defer blkHistoryMutex.RUnlock()

		cpyHistory := make([]*models.Block, len(blkHistory))
		copy(cpyHistory, blkHistory)

		var res []vertex
		for _, block := range cpyHistory {
			res = append(res, vertex{
				ID:              block.ID().Base58(),
				ParentIDsByType: prepareParentReferences(block),
				IsFinalized:     blkFinalized[block.ID().Base58()],
				IsTx:            block.Payload().Type() == devnetvm.TransactionType,
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(blk *models.Block, finalized bool) {
	blkHistoryMutex.Lock()
	defer blkHistoryMutex.Unlock()
	if _, exist := blkFinalized[blk.ID().Base58()]; exist {
		blkFinalized[blk.ID().Base58()] = finalized
		return
	}

	// remove 100 old blks if the slice is full
	if len(blkHistory) >= maxBlkHistorySize {
		for i := 0; i < numHistoryToRemove; i++ {
			delete(blkFinalized, blkHistory[i].ID().Base58())
		}
		blkHistory = append(blkHistory[:0], blkHistory[numHistoryToRemove:maxBlkHistorySize]...)
	}
	// add new blk
	blkHistory = append(blkHistory, blk)
	blkFinalized[blk.ID().Base58()] = finalized
}
