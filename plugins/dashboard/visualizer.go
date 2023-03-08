package dashboard

import (
	"context"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

var (
	currentSlot atomic.Int64
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string              `json:"id"`
	ParentIDsByType map[string][]string `json:"parentIDsByType"`
	IsFinalized     bool                `json:"is_finalized"`
	IsTx            bool                `json:"is_tx"`
	IssuingTime     time.Time
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
				sendVertex(block.ModelsBlock, false)
				if block.ID().Index() > slot.Index(currentSlot.Load()) {
					currentSlot.Store(int64(block.ID().Index()))
				}
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
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
		var res []vertex

		start := slot.Index(currentSlot.Load())
		for _, ei := range []slot.Index{start - 1, start} {
			blocks := deps.Retainer.LoadAllBlockMetadata(ei)
			_ = blocks.ForEach(func(element *retainer.BlockMetadata) (err error) {
				res = append(res, vertex{
					ID:              element.ID().Base58(),
					ParentIDsByType: prepareParentReferences(element.M.Block),
					IsFinalized:     element.M.Accepted,
					IsTx:            element.M.Block.Payload().Type() == devnetvm.TransactionType,
					IssuingTime:     element.M.Block.IssuingTime(),
				})
				return
			})
		}

		sort.Slice(res, func(i, j int) bool {
			return res[i].IssuingTime.Before(res[j].IssuingTime)
		})

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}
