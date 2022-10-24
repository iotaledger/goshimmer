package dagsvisualizer

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	maxWsBlockBufferSize      = 200
	buffer                    []*wsBlock
	bufferMutex               sync.RWMutex
)

func setupVisualizer() {
	// create and start workerpool
	visualizerWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		broadcastWsBlock(task.Param(0))
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))
}

func runVisualizer() {
	if err := daemon.BackgroundWorker("Dags Visualizer[Visualizer]", func(ctx context.Context) {
		// register to events
		registerTangleEvents()
		registerUTXOEvents()
		registerConflictEvents()

		<-ctx.Done()
		log.Info("Stopping DAGs Visualizer ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping DAGs Visualizer ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerTangleEvents() {
	storeClosure := event.NewClosure(func(block *blockdag.Block) {
		wsBlk := &wsBlock{
			Type: BlkTypeTangleVertex,
			Data: newTangleVertex(block.ModelsBlock),
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	bookedClosure := event.NewClosure(func(block *booker.Block) {
		conflictIDs := deps.Protocol.Engine().Tangle.BlockConflicts(block)

		wsBlk := &wsBlock{
			Type: BlkTypeTangleBooked,
			Data: &tangleBooked{
				ID:          block.ID().Base58(),
				IsMarker:    block.StructureDetails().IsPastMarker(),
				ConflictIDs: lo.Map(conflictIDs.Slice(), utxo.TransactionID.Base58),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	blkAcceptedClosure := event.NewClosure(func(block *acceptance.Block) {
		wsBlk := &wsBlock{
			Type: BlkTypeTangleConfirmed,
			Data: &tangleConfirmed{
				ID:           block.ID().Base58(),
				Accepted:     block.IsAccepted(),
				AcceptedTime: time.Now().UnixNano(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	txAcceptedClosure := event.NewClosure(func(txMeta *ledger.TransactionMetadata) {
		attachmentBlock := deps.Protocol.Engine().Tangle.GetEarliestAttachment(txMeta.ID())

		wsBlk := &wsBlock{
			Type: BlkTypeTangleTxConfirmationState,
			Data: &tangleTxConfirmationStateChanged{
				ID:          attachmentBlock.ID().Base58(),
				IsConfirmed: deps.Protocol.Engine().Ledger.Utils.TransactionConfirmationState(txMeta.ID()).IsAccepted(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(storeClosure)
	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(bookedClosure)
	deps.Protocol.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(blkAcceptedClosure)
	deps.Protocol.Events.Engine.Ledger.TransactionAccepted.Attach(txAcceptedClosure)
}

func registerUTXOEvents() {
	storeClosure := event.NewClosure(func(block *blockdag.Block) {
		if block.Payload().Type() == devnetvm.TransactionType {
			tx := block.Payload().(*devnetvm.Transaction)
			wsBlk := &wsBlock{
				Type: BlkTypeUTXOVertex,
				Data: newUTXOVertex(block.ID(), tx),
			}
			visualizerWorkerPool.TrySubmit(wsBlk)
			storeWsBlock(wsBlk)
		}
	})

	bookedClosure := event.NewClosure(func(block *booker.Block) {
		if block.Payload().Type() == devnetvm.TransactionType {
			tx := block.Payload().(*devnetvm.Transaction)
			deps.Protocol.Engine().Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
				wsBlk := &wsBlock{
					Type: BlkTypeUTXOBooked,
					Data: &utxoBooked{
						ID:          tx.ID().Base58(),
						ConflictIDs: lo.Map(txMetadata.ConflictIDs().Slice(), utxo.TransactionID.Base58),
					},
				}
				visualizerWorkerPool.TrySubmit(wsBlk)
				storeWsBlock(wsBlk)
			})
		}
	})

	txAcceptedClosure := event.NewClosure(func(txMeta *ledger.TransactionMetadata) {
		wsBlk := &wsBlock{
			Type: BlkTypeUTXOConfirmationStateChanged,
			Data: &utxoConfirmationStateChanged{
				ID:                    txMeta.ID().Base58(),
				ConfirmationState:     txMeta.ConfirmationState().String(),
				ConfirmationStateTime: txMeta.ConfirmationStateTime().UnixNano(),
				IsConfirmed:           txMeta.ConfirmationState().IsAccepted(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(storeClosure)
	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(bookedClosure)
	deps.Protocol.Events.Engine.Ledger.TransactionAccepted.Attach(txAcceptedClosure)
}

func registerConflictEvents() {
	createdClosure := event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		wsBlk := &wsBlock{
			Type: BlkTypeConflictVertex,
			Data: newConflictVertex(event.ID),
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	parentUpdateClosure := event.NewClosure(func(event *conflictdag.ConflictParentsUpdatedEvent[utxo.TransactionID, utxo.OutputID]) {
		lo.Map(event.ParentsConflictIDs.Slice(), utxo.TransactionID.Base58)
		wsBlk := &wsBlock{
			Type: BlkTypeConflictParentsUpdate,
			Data: &conflictParentUpdate{
				ID:      event.ConflictID.Base58(),
				Parents: lo.Map(event.ParentsConflictIDs.Slice(), utxo.TransactionID.Base58),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	conflictConfirmedClosure := event.NewClosure(func(conflictID utxo.TransactionID) {
		wsBlk := &wsBlock{
			Type: BlkTypeConflictConfirmationStateChanged,
			Data: &conflictConfirmationStateChanged{
				ID:                conflictID.Base58(),
				ConfirmationState: confirmation.Accepted.String(),
				IsConfirmed:       true,
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	conflictWeightChangedClosure := event.NewClosure(func(e *conflicttracker.VoterEvent[utxo.TransactionID]) {
		conflictConfirmationState := deps.Protocol.Engine().Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(e.ConflictID))
		voters := deps.Protocol.Engine().Tangle.VirtualVoting.ConflictVoters(e.ConflictID)
		wsBlk := &wsBlock{
			Type: BlkTypeConflictWeightChanged,
			Data: &conflictWeightChanged{
				ID:                e.ConflictID.Base58(),
				Weight:            voters.TotalWeight(),
				ConfirmationState: conflictConfirmationState.String(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictCreated.Attach(createdClosure)
	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Attach(conflictConfirmedClosure)
	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictParentsUpdated.Attach(parentUpdateClosure)
	deps.Protocol.Events.Engine.Tangle.VirtualVoting.ConflictTracker.VoterAdded.Attach(conflictWeightChangedClosure)
	deps.Protocol.Events.Engine.Tangle.VirtualVoting.ConflictTracker.VoterRemoved.Attach(conflictWeightChangedClosure)
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
		// startEpoch := epoch.IndexFromTime(startTimestamp)
		// endEpoch := epoch.IndexFromTime(endTimestamp)
		//
		// var blocks []*tangleVertex
		// var txs []*utxoVertex
		// var conflicts []*conflictVertex
		// conflictMap := set.NewAdvancedSet[utxo.TransactionID]()
		// entryBlks := models.NewBlockIDs()
		// TODO: use retainer and optimize to use data locality

		// deps.Tangle.Utils.WalkBlockID(func(blockID tangleold.BlockID, walker *walker.Walker[tangleold.BlockID]) {
		//	deps.Tangle.Storage.Block(blockID).Consume(func(blk *tangleold.Block) {
		//		// only keep blocks that is issued in the given time interval
		//		if blk.IssuingTime().After(startTimestamp) && blk.IssuingTime().Before(endTimestamp) {
		//			// add block
		//			tangleNode := newTangleVertex(blk)
		//			blocks = append(blocks, tangleNode)
		//
		//			// add tx
		//			if tangleNode.IsTx {
		//				utxoNode := newUTXOVertex(blk.ID(), blk.Payload().(*devnetvm.Transaction))
		//				txs = append(txs, utxoNode)
		//			}
		//
		//			// add conflict
		//			conflictIDs, err := deps.Tangle.Booker.BlockConflictIDs(blk.ID())
		//			if err != nil {
		//				conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
		//			}
		//			for it := conflictIDs.Iterator(); it.HasNext(); {
		//				conflictID := it.Next()
		//				if conflictMap.Has(conflictID) {
		//					continue
		//				}
		//
		//				conflictMap.Add(conflictID)
		//				conflicts = append(conflicts, newConflictVertex(conflictID))
		//			}
		//		}
		//
		//		// continue walking if the block is issued before endTimestamp
		//		if blk.IssuingTime().Before(endTimestamp) {
		//			deps.Tangle.Storage.Children(blockID).Consume(func(child *tangleold.Child) {
		//				walker.Push(child.ChildBlockID())
		//			})
		//		}
		//	})
		// }, entryBlks)

		// return c.JSON(http.StatusOK, searchResult{Blocks: blocks, Txs: txs, Conflicts: conflicts})
		return c.JSON(http.StatusBadRequest, searchResult{Error: "invalid timestamp range"})
	})
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
	deps.Protocol.Engine().Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
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
	deps.Protocol.Engine().Ledger.ConflictDAG.Storage.CachedConflict(conflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		conflicts := make(map[utxo.OutputID][]utxo.TransactionID)
		// get conflicts of a conflict
		for it := conflict.ConflictSetIDs().Iterator(); it.HasNext(); {
			conflictID := it.Next()
			conflicts[conflictID] = make([]utxo.TransactionID, 0)
			deps.Protocol.Engine().Ledger.ConflictDAG.Storage.CachedConflictMembers(conflictID).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {
				conflicts[conflictID] = append(conflicts[conflictID], conflictMember.ConflictID())
			})
		}
		confirmationState := deps.Protocol.Engine().Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(conflictID))
		ret = &conflictVertex{
			ID:                conflictID.Base58(),
			Parents:           lo.Map(conflict.Parents().Slice(), utxo.TransactionID.Base58),
			Conflicts:         jsonmodels.NewGetConflictConflictsResponse(conflict.ID(), conflicts),
			IsConfirmed:       confirmationState.IsAccepted(),
			ConfirmationState: confirmationState.String(),
			AW:                deps.Protocol.Engine().Tangle.VirtualVoting.ConflictVoters(conflictID).TotalWeight(),
		}
	})
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
