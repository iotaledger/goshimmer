package dashboard

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
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
	visualizerWorkerCount     = 4
	visualizerWorkerQueueSize = 1000
	visualizerWorkerPool      *workerpool.NonBlockingQueuedWorkerPool

	blkHistoryMutex    sync.RWMutex
	blkState           map[string]vertexMeta
	blkHistory         []*models.Block
	maxBlkHistorySize  = 1000
	numHistoryToRemove = 100
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string              `json:"id"`
	ParentIDsByType map[string][]string `json:"parentIDsByType"`
	IsAccepted      bool                `json:"isAccepted"`
	IsOrphaned      bool                `json:"isOrphaned"`
	IsMissing       bool                `json:"isMissing"`
	IsInvalid       bool                `json:"isInvalid"`
	IsSolid         bool                `json:"isSolid"`
	ISBooked        bool                `json:"isBooked"`
	IsFinalized     bool                `json:"isFinalized"`
	IsTx            bool                `json:"isTx"`
	IsTxConflicting bool                `json:"isTxConflicting"`
	IsTxRejected    bool                `json:"isTxRejected"`
	IsTxAccepted    bool                `json:"isTxAccepted"`
}

type vertexMeta struct {
	IsTx            bool `json:"isTx"`
	IsTxConflicting bool `json:"isTxConflicting"`
	IsTxRejected    bool `json:"isTxRejected"`
	IsTxAccepted    bool `json:"isTxAccepted"`
	IsAccepted      bool `json:"isAccepted"`
	IsOrphaned      bool `json:"isOrphaned"`
	IsMissing       bool `json:"isMissing"`
	IsInvalid       bool `json:"isInvalid"`
	IsSolid         bool `json:"isSolid"`
	ISBooked        bool `json:"isBooked"`
	IsFinalized     bool `json:"isFinalized"`
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
			sendVertex(x, task.Param(1).(vertexMeta))
		case *scheduler.Block:
			sendTipInfo(x, true)
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure blkHistory, blkSolid
	blkState = make(map[string]vertexMeta, maxBlkHistorySize)
	blkHistory = make([]*models.Block, 0, maxBlkHistorySize)
}

func sendVertex(blk *models.Block, meta vertexMeta) {
	broadcastWsBlock(&wsblk{MsgTypeVertex, &vertex{
		ID:              blk.ID().Base58(),
		ParentIDsByType: prepareParentReferences(blk),
		IsAccepted:      meta.IsAccepted,
		IsOrphaned:      meta.IsOrphaned,
		IsMissing:       meta.IsMissing,
		IsInvalid:       meta.IsInvalid,
		IsSolid:         meta.IsSolid,
		ISBooked:        meta.ISBooked,
		IsFinalized:     meta.IsFinalized,
		IsTx:            blk.Payload().Type() == devnetvm.TransactionType,
		IsTxConflicting: meta.IsTxConflicting,
		IsTxRejected:    meta.IsTxRejected,
		IsTxAccepted:    meta.IsTxAccepted,
	}}, true)
}

func sendTipInfo(block *scheduler.Block, isTip bool) {
	broadcastWsBlock(&wsblk{MsgTypeTipInfo, &tipinfo{
		ID:    block.ID().Base58(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	processBlock := func(block *models.Block, state vertexMeta) {
		addToHistory(block, state)
		visualizerWorkerPool.TrySubmit(block, state)
	}

	notifyNewBlkStored := event.NewClosure(func(block *blockdag.Block) {
		processBlock(block.ModelsBlock, vertexMeta{
			IsOrphaned: block.IsOrphaned(),
			IsMissing:  block.IsMissing(),
			IsInvalid:  block.IsInvalid(),
			IsSolid:    block.IsSolid(),
		})
	})

	notifyNewBlkAccepted := event.NewClosure(func(block *blockgadget.Block) {
		processBlock(block.ModelsBlock, vertexMeta{
			IsAccepted: block.IsAccepted(),
			IsOrphaned: block.IsOrphaned(),
			IsMissing:  block.IsMissing(),
			IsInvalid:  block.IsInvalid(),
			IsSolid:    block.IsSolid(),
			ISBooked:   block.IsBooked(),
		})
	})

	notifyNewBlkConfirmed := event.NewClosure(func(block *blockgadget.Block) {
		processBlock(block.ModelsBlock, vertexMeta{
			IsAccepted:  block.IsAccepted(),
			IsOrphaned:  block.IsOrphaned(),
			IsMissing:   block.IsMissing(),
			IsInvalid:   block.IsInvalid(),
			IsSolid:     block.IsSolid(),
			ISBooked:    block.IsBooked(),
			IsFinalized: block.IsConfirmed(),
		})
	})

	notifyNewTip := event.NewClosure(func(block *scheduler.Block) {
		visualizerWorkerPool.TrySubmit(block.ModelsBlock, vertexMeta{
			IsOrphaned: block.IsOrphaned(),
			IsMissing:  block.IsMissing(),
			IsInvalid:  block.IsInvalid(),
			IsSolid:    block.IsSolid(),
			ISBooked:   block.IsBooked(),
		})
	})

	notifyDeletedTip := event.NewClosure(func(block *scheduler.Block) {
		visualizerWorkerPool.TrySubmit(block.ModelsBlock, vertexMeta{
			IsOrphaned: block.IsOrphaned(),
			IsMissing:  block.IsMissing(),
			IsInvalid:  block.IsInvalid(),
			IsSolid:    block.IsSolid(),
			ISBooked:   block.IsBooked(),
		})
	})

	txForked := event.NewClosure(func(event *ledger.TransactionForkedEvent) {
		attachments := deps.Protocol.Engine().Tangle.GetAllAttachments(event.TransactionID)
		attachments.ForEach(func(block *booker.Block) error {
			visualizerWorkerPool.TrySubmit(block.ModelsBlock, vertexMeta{
				IsOrphaned:      block.IsOrphaned(),
				IsMissing:       block.IsMissing(),
				IsInvalid:       block.IsInvalid(),
				IsSolid:         block.IsSolid(),
				ISBooked:        block.IsBooked(),
				IsTx:            true,
				IsTxConflicting: true,
			})
			return nil
		})
	})

	txAccepted := event.NewClosure(func(event *ledger.TransactionEvent) {
		attachments := deps.Protocol.Engine().Tangle.GetAllAttachments(event.Metadata.ID())
		attachments.ForEach(func(block *booker.Block) error {
			visualizerWorkerPool.TrySubmit(block.ModelsBlock, vertexMeta{
				IsOrphaned:   block.IsOrphaned(),
				IsMissing:    block.IsMissing(),
				IsInvalid:    block.IsInvalid(),
				IsSolid:      block.IsSolid(),
				ISBooked:     block.IsBooked(),
				IsTx:         true,
				IsTxAccepted: true,
			})
			return nil
		})
	})

	txRejected := event.NewClosure(func(event *ledger.TransactionMetadata) {
		attachments := deps.Protocol.Engine().Tangle.GetAllAttachments(event.ID())
		attachments.ForEach(func(block *booker.Block) error {
			visualizerWorkerPool.TrySubmit(block.ModelsBlock, vertexMeta{
				IsOrphaned:   block.IsOrphaned(),
				IsMissing:    block.IsMissing(),
				IsInvalid:    block.IsInvalid(),
				IsSolid:      block.IsSolid(),
				ISBooked:     block.IsBooked(),
				IsTx:         true,
				IsTxRejected: true,
			})
			return nil
		})
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		deps.Protocol.Events.Engine.Ledger.TransactionForked.Attach(txForked)
		defer deps.Protocol.Events.Engine.Ledger.TransactionForked.Detach(txForked)
		deps.Protocol.Events.Engine.Ledger.TransactionAccepted.Attach(txAccepted)
		defer deps.Protocol.Events.Engine.Ledger.TransactionAccepted.Detach(txAccepted)
		deps.Protocol.Events.Engine.Ledger.TransactionRejected.Attach(txRejected)
		defer deps.Protocol.Events.Engine.Ledger.TransactionRejected.Detach(txRejected)

		deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(notifyNewBlkStored)
		defer deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Detach(notifyNewBlkStored)
		deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(notifyNewBlkAccepted)
		defer deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Detach(notifyNewBlkAccepted)
		deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Attach(notifyNewBlkConfirmed)
		defer deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Detach(notifyNewBlkConfirmed)
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
			meta := blkState[block.ID().Base58()]
			res = append(res, vertex{
				ID:              block.ID().Base58(),
				ParentIDsByType: prepareParentReferences(block),
				IsAccepted:      meta.IsAccepted,
				IsOrphaned:      meta.IsOrphaned,
				IsMissing:       meta.IsMissing,
				IsInvalid:       meta.IsInvalid,
				IsSolid:         meta.IsSolid,
				ISBooked:        meta.ISBooked,
				IsFinalized:     meta.IsFinalized,
				IsTx:            block.Payload().Type() == devnetvm.TransactionType,
				IsTxConflicting: meta.IsTxConflicting,
				IsTxRejected:    meta.IsTxRejected,
				IsTxAccepted:    meta.IsTxAccepted,
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(blk *models.Block, state vertexMeta) {
	blkHistoryMutex.Lock()
	defer blkHistoryMutex.Unlock()
	if _, exist := blkState[blk.ID().Base58()]; exist {
		blkState[blk.ID().Base58()] = state
		return
	}

	// remove 100 old blks if the slice is full
	if len(blkHistory) >= maxBlkHistorySize {
		for i := 0; i < numHistoryToRemove; i++ {
			delete(blkState, blkHistory[i].ID().Base58())
		}
		blkHistory = append(blkHistory[:0], blkHistory[numHistoryToRemove:maxBlkHistorySize]...)
	}

	// add new blk
	blkHistory = append(blkHistory, blk)
	blkState[blk.ID().Base58()] = state
}
