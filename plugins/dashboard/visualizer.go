package dashboard

import (
	"context"
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.NonBlockingQueuedWorkerPool

	blkHistoryMutex    sync.RWMutex
	blkFinalized       map[string]bool
	blkHistory         []*tangleold.Block
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
		case *tangleold.Block:
			sendVertex(x, task.Param(1).(bool))
		case *tangleold.TipEvent:
			sendTipInfo(task.Param(1).(tangleold.BlockID), task.Param(2).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure blkHistory, blkSolid
	blkFinalized = make(map[string]bool, maxBlkHistorySize)
	blkHistory = make([]*tangleold.Block, 0, maxBlkHistorySize)
}

func sendVertex(blk *tangleold.Block, finalized bool) {
	broadcastWsBlock(&wsblk{MsgTypeVertex, &vertex{
		ID:              blk.ID().Base58(),
		ParentIDsByType: prepareParentReferences(blk),
		IsFinalized:     finalized,
		IsTx:            blk.Payload().Type() == devnetvm.TransactionType,
	}}, true)
}

func sendTipInfo(blockID tangleold.BlockID, isTip bool) {
	broadcastWsBlock(&wsblk{MsgTypeTipInfo, &tipinfo{
		ID:    blockID.Base58(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	processBlock := func(block *tangleold.Block) {
		finalized := deps.Tangle.ConfirmationOracle.IsBlockConfirmed(block.ID())
		addToHistory(block, finalized)
		visualizerWorkerPool.TrySubmit(block, finalized)
	}

	notifyNewBlkStored := event.NewClosure(func(event *tangleold.BlockStoredEvent) {
		processBlock(event.Block)
	})

	notifyNewBlkAccepted := event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
		processBlock(event.Block)
	})

	notifyNewTip := event.NewClosure(func(tipEvent *tangleold.TipEvent) {
		visualizerWorkerPool.TrySubmit(tipEvent, tipEvent.BlockID, true)
	})

	notifyDeletedTip := event.NewClosure(func(tipEvent *tangleold.TipEvent) {
		visualizerWorkerPool.TrySubmit(tipEvent, tipEvent.BlockID, false)
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		deps.Tangle.Storage.Events.BlockStored.Attach(notifyNewBlkStored)
		defer deps.Tangle.Storage.Events.BlockStored.Detach(notifyNewBlkStored)
		deps.Tangle.ConfirmationOracle.Events().BlockAccepted.Attach(notifyNewBlkAccepted)
		defer deps.Tangle.ConfirmationOracle.Events().BlockAccepted.Detach(notifyNewBlkAccepted)
		deps.Tangle.TipManager.Events.TipAdded.Attach(notifyNewTip)
		defer deps.Tangle.TipManager.Events.TipAdded.Detach(notifyNewTip)
		deps.Tangle.TipManager.Events.TipRemoved.Attach(notifyDeletedTip)
		defer deps.Tangle.TipManager.Events.TipRemoved.Detach(notifyDeletedTip)
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

		cpyHistory := make([]*tangleold.Block, len(blkHistory))
		copy(cpyHistory, blkHistory)

		var res []vertex
		for _, blk := range cpyHistory {
			res = append(res, vertex{
				ID:              blk.ID().Base58(),
				ParentIDsByType: prepareParentReferences(blk),
				IsFinalized:     blkFinalized[blk.ID().Base58()],
				IsTx:            blk.Payload().Type() == devnetvm.TransactionType,
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(blk *tangleold.Block, finalized bool) {
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
