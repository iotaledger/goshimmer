package dagsvisualizer

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

var (
	maxWsBlockBufferSize = 200
	buffer               []*wsBlock
	bufferMutex          sync.RWMutex
)

func runVisualizer(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dags Visualizer[Visualizer]", func(ctx context.Context) {
		// register to events
		registerTangleEvents(plugin)
		registerUTXOEvents(plugin)
		registerConflictEvents(plugin)

		<-ctx.Done()
		log.Info("Stopping DAGs Visualizer ...")
		log.Info("Stopping DAGs Visualizer ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerTangleEvents(plugin *node.Plugin) {
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
		wsBlk := &wsBlock{
			Type: BlkTypeTangleVertex,
			Data: newTangleVertex(block.ModelsBlock),
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Hook(func(evt *booker.BlockBookedEvent) {
		conflictIDs := deps.Protocol.Engine().Tangle.Booker().BlockConflicts(evt.Block)

		wsBlk := &wsBlock{
			Type: BlkTypeTangleBooked,
			Data: &tangleBooked{
				ID:          evt.Block.ID().Base58(),
				IsMarker:    evt.Block.StructureDetails().IsPastMarker(),
				ConflictIDs: lo.Map(conflictIDs.Slice(), utxo.TransactionID.Base58),
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		wsBlk := &wsBlock{
			Type: BlkTypeTangleConfirmed,
			Data: &tangleConfirmed{
				ID:           block.ID().Base58(),
				Accepted:     block.IsAccepted(),
				AcceptedTime: time.Now().UnixNano(),
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Ledger.MemPool.TransactionAccepted.Hook(func(event *mempool.TransactionEvent) {
		attachmentBlock := deps.Protocol.Engine().Tangle.Booker().GetEarliestAttachment(event.Metadata.ID())

		wsBlk := &wsBlock{
			Type: BlkTypeTangleTxConfirmationState,
			Data: &tangleTxConfirmationStateChanged{
				ID:          attachmentBlock.ID().Base58(),
				IsConfirmed: deps.Protocol.Engine().Ledger.MemPool().Utils().TransactionConfirmationState(event.Metadata.ID()).IsAccepted(),
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func registerUTXOEvents(plugin *node.Plugin) {
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
		if block.Payload().Type() == devnetvm.TransactionType {
			tx := block.Payload().(*devnetvm.Transaction)
			wsBlk := &wsBlock{
				Type: BlkTypeUTXOVertex,
				Data: newUTXOVertex(block.ID(), tx),
			}
			broadcastWsBlock(wsBlk)
			storeWsBlock(wsBlk)
		}
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Hook(func(evt *booker.BlockBookedEvent) {
		if evt.Block.Payload().Type() == devnetvm.TransactionType {
			tx := evt.Block.Payload().(*devnetvm.Transaction)
			deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *mempool.TransactionMetadata) {
				wsBlk := &wsBlock{
					Type: BlkTypeUTXOBooked,
					Data: &utxoBooked{
						ID:          tx.ID().Base58(),
						ConflictIDs: lo.Map(txMetadata.ConflictIDs().Slice(), utxo.TransactionID.Base58),
					},
				}
				broadcastWsBlock(wsBlk)
				storeWsBlock(wsBlk)
			})
		}
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Ledger.MemPool.TransactionAccepted.Hook(func(event *mempool.TransactionEvent) {
		txMeta := event.Metadata
		wsBlk := &wsBlock{
			Type: BlkTypeUTXOConfirmationStateChanged,
			Data: &utxoConfirmationStateChanged{
				ID:                    txMeta.ID().Base58(),
				ConfirmationState:     txMeta.ConfirmationState().String(),
				ConfirmationStateTime: txMeta.ConfirmationStateTime().UnixNano(),
				IsConfirmed:           txMeta.ConfirmationState().IsAccepted(),
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func registerConflictEvents(plugin *node.Plugin) {
	conflictWeightChangedFunc := func(e *conflicttracker.VoterEvent[utxo.TransactionID]) {
		conflictConfirmationState := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ConfirmationState(utxo.NewTransactionIDs(e.ConflictID))
		wsBlk := &wsBlock{
			Type: BlkTypeConflictWeightChanged,
			Data: &conflictWeightChanged{
				ID:                e.ConflictID.Base58(),
				Weight:            deps.Protocol.Engine().Tangle.Booker().VirtualVoting().ConflictVotersTotalWeight(e.ConflictID),
				ConfirmationState: conflictConfirmationState.String(),
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}

	deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictCreated.Hook(func(event *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		wsBlk := &wsBlock{
			Type: BlkTypeConflictVertex,
			Data: newConflictVertex(event.ID()),
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		wsBlk := &wsBlock{
			Type: BlkTypeConflictConfirmationStateChanged,
			Data: &conflictConfirmationStateChanged{
				ID:                conflict.ID().Base58(),
				ConfirmationState: confirmation.Accepted.String(),
				IsConfirmed:       true,
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictParentsUpdated.Hook(func(event *conflictdag.ConflictParentsUpdatedEvent[utxo.TransactionID, utxo.OutputID]) {
		lo.Map(event.ParentsConflictIDs.Slice(), utxo.TransactionID.Base58)
		wsBlk := &wsBlock{
			Type: BlkTypeConflictParentsUpdate,
			Data: &conflictParentUpdate{
				ID:      event.ConflictID.Base58(),
				Parents: lo.Map(event.ParentsConflictIDs.Slice(), utxo.TransactionID.Base58),
			},
		}
		broadcastWsBlock(wsBlk)
		storeWsBlock(wsBlk)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Tangle.Booker.VirtualVoting.ConflictTracker.VoterAdded.Hook(conflictWeightChangedFunc, event.WithWorkerPool(plugin.WorkerPool))
	deps.Protocol.Events.Engine.Tangle.Booker.VirtualVoting.ConflictTracker.VoterRemoved.Hook(conflictWeightChangedFunc, event.WithWorkerPool(plugin.WorkerPool))
}

func setupDagsVisualizerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/dagsvisualizer/conflict/:conflictID", func(c echo.Context) (err error) {
		parents := make(map[string]*conflictVertex)
		var conflictID utxo.TransactionID
		if err = conflictID.FromBase58(c.Param("conflictID")); err != nil {
			err = c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
			return
		}
		vertex := newConflictVertex(conflictID)
		parents[vertex.ID] = vertex
		getConflictsToMaster(vertex, parents)

		var conflicts []*conflictVertex
		for _, conflict := range parents {
			conflicts = append(conflicts, conflict)
		}
		return c.JSON(http.StatusOK, conflicts)
	})

	routeGroup.GET("/dagsvisualizer/search/:start/:end", func(c echo.Context) (err error) {
		startTimestamp := parseStringToTimestamp(c.Param("start"))
		endTimestamp := parseStringToTimestamp(c.Param("end"))

		reqValid := isTimeIntervalValid(startTimestamp, endTimestamp)
		if !reqValid {
			return c.JSON(http.StatusBadRequest, searchResult{Error: "invalid timestamp range"})
		}
		startSlot := deps.Protocol.SlotTimeProvider().IndexFromTime(startTimestamp)
		endSlot := deps.Protocol.SlotTimeProvider().IndexFromTime(endTimestamp)

		var blocks []*tangleVertex
		var txs []*utxoVertex
		var conflicts []*conflictVertex
		conflictMap := utxo.NewTransactionIDs()
		for i := startSlot; i <= endSlot; i++ {
			deps.Retainer.StreamBlocksMetadata(i, func(id models.BlockID, metadata *retainer.BlockMetadata) {
				if metadata.M.Block.IssuingTime().After(startTimestamp) && metadata.M.Block.IssuingTime().Before(endTimestamp) {
					tangleNode, utxoNode, blockConflicts := processMetadata(metadata, conflictMap)
					blocks = append(blocks, tangleNode)
					if utxoNode != nil {
						txs = append(txs, utxoNode)
					}
					conflicts = append(conflicts, blockConflicts...)
				}
			})
		}
		return c.JSON(http.StatusOK, searchResult{Blocks: blocks, Txs: txs, Conflicts: conflicts})
	})
}

func processMetadata(metadata *retainer.BlockMetadata, conflictMap utxo.TransactionIDs) (tangleNode *tangleVertex, utxoNode *utxoVertex, conflicts []*conflictVertex) {
	// add block
	tangleNode = newTangleVertex(metadata.M.Block)

	// add tx
	if tangleNode.IsTx {
		utxoNode = newUTXOVertex(metadata.ID(), metadata.M.Block.Payload().(*devnetvm.Transaction))
	}

	// add conflict
	if metadata.M.ConflictIDs != nil {
		for it := metadata.M.ConflictIDs.Iterator(); it.HasNext(); {
			conflictID := it.Next()
			if conflictMap.Add(conflictID) {
				conflicts = append(conflicts, newConflictVertex(conflictID))
			}
		}
	}

	return tangleNode, utxoNode, conflicts
}

func parseStringToTimestamp(str string) (t time.Time) {
	ts, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}

func isTimeIntervalValid(start, end time.Time) (valid bool) {
	if start.IsZero() || end.IsZero() {
		return false
	}

	if start.After(end) {
		return false
	}

	return true
}

func newTangleVertex(block *models.Block) (ret *tangleVertex) {
	confirmationState := confirmation.Pending

	ret = &tangleVertex{
		ID:                    block.ID().Base58(),
		StrongParentIDs:       block.ParentsByType(models.StrongParentType).Base58(),
		WeakParentIDs:         block.ParentsByType(models.WeakParentType).Base58(),
		ShallowLikeParentIDs:  block.ParentsByType(models.ShallowLikeParentType).Base58(),
		IsTx:                  block.Payload().Type() == devnetvm.TransactionType,
		IsConfirmed:           false,
		ConfirmationStateTime: block.IssuingTime().Unix(),
		ConfirmationState:     confirmationState.String(),
	}

	if ret.IsTx {
		ret.TxID = block.Payload().(*devnetvm.Transaction).ID().Base58()
	}
	return
}

func newUTXOVertex(blkID models.BlockID, tx *devnetvm.Transaction) (ret *utxoVertex) {
	inputs := make([]*jsonmodels.Input, len(tx.Essence().Inputs()))
	for i, input := range tx.Essence().Inputs() {
		inputs[i] = jsonmodels.NewInput(input)
	}

	outputs := make([]string, len(tx.Essence().Outputs()))
	for i, output := range tx.Essence().Outputs() {
		outputs[i] = output.ID().Base58()
	}

	var confirmationState confirmation.State
	var confirmedTime int64
	var conflictIDs []string
	deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *mempool.TransactionMetadata) {
		confirmationState = txMetadata.ConfirmationState()
		confirmedTime = txMetadata.ConfirmationStateTime().UnixNano()
		conflictIDs = lo.Map(txMetadata.ConflictIDs().Slice(), utxo.TransactionID.Base58)
	})

	ret = &utxoVertex{
		BlkID:                 blkID.Base58(),
		ID:                    tx.ID().Base58(),
		Inputs:                inputs,
		Outputs:               outputs,
		IsConfirmed:           confirmationState.IsAccepted(),
		ConflictIDs:           conflictIDs,
		ConfirmationState:     confirmationState.String(),
		ConfirmationStateTime: confirmedTime,
	}

	return ret
}

func newConflictVertex(conflictID utxo.TransactionID) (ret *conflictVertex) {
	conflict, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID)
	if !exists {
		return
	}
	conflicts := make(map[utxo.OutputID][]utxo.TransactionID)
	// get conflicts of a conflict
	for it := conflict.ConflictSets().Iterator(); it.HasNext(); {
		conflictSet := it.Next()
		conflicts[conflictSet.ID()] = make([]utxo.TransactionID, 0)

		conflicts[conflictSet.ID()] = lo.Map(conflictSet.Conflicts().Slice(), func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) utxo.TransactionID {
			return conflict.ID()
		})
	}
	confirmationState := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ConfirmationState(utxo.NewTransactionIDs(conflictID))
	ret = &conflictVertex{
		ID:                conflictID.Base58(),
		Parents:           lo.Map(conflict.Parents().Slice(), utxo.TransactionID.Base58),
		Conflicts:         jsonmodels.NewGetConflictConflictsResponse(conflict.ID(), conflicts),
		IsConfirmed:       confirmationState.IsAccepted(),
		ConfirmationState: confirmationState.String(),
		AW:                deps.Protocol.Engine().Tangle.Booker().VirtualVoting().ConflictVotersTotalWeight(conflictID),
	}
	return
}

func storeWsBlock(blk *wsBlock) {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()
	if len(buffer) >= maxWsBlockBufferSize {
		buffer = buffer[1:]
	}
	buffer = append(buffer, blk)
}

func getConflictsToMaster(vertex *conflictVertex, parents map[string]*conflictVertex) {
	for _, IDBase58 := range vertex.Parents {
		if _, ok := parents[IDBase58]; !ok {
			var ID utxo.TransactionID
			if err := ID.FromBase58(IDBase58); err == nil {
				parentVertex := newConflictVertex(ID)
				parents[parentVertex.ID] = parentVertex
				getConflictsToMaster(parentVertex, parents)
			}
		}
	}
}
