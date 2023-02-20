package dashboard

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

var (
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

func runVisualizer(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		unhook := lo.Batch(
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
				addToHistory(block.ModelsBlock, false)
				sendVertex(block.ModelsBlock, false)
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				addToHistory(block.ModelsBlock, block.IsAccepted())
				sendVertex(block.ModelsBlock, block.IsAccepted())
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
			deps.Protocol.Events.TipManager.TipAdded.Hook(func(block *scheduler.Block) {
				sendTipInfo(block, true)
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
			deps.Protocol.Events.TipManager.TipRemoved.Hook(func(block *scheduler.Block) {
				sendTipInfo(block, false)
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
		)
		<-ctx.Done()
		log.Info("Stopping Dashboard[Visualizer] ...")
		unhook()
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
