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
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
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
	storeClosure := event.NewClosure(func(event *tangleold.BlockStoredEvent) {
		wsBlk := &wsBlock{
			Type: BlkTypeTangleVertex,
			Data: newTangleVertex(event.Block),
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	bookedClosure := event.NewClosure(func(event *tangleold.BlockBookedEvent) {
		blockID := event.BlockID
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetadata *tangleold.BlockMetadata) {
			conflictIDs, err := deps.Tangle.Booker.BlockConflictIDs(blockID)
			if err != nil {
				conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
			}

			wsBlk := &wsBlock{
				Type: BlkTypeTangleBooked,
				Data: &tangleBooked{
					ID:          blockID.Base58(),
					IsMarker:    blkMetadata.StructureDetails().IsPastMarker(),
					ConflictIDs: lo.Map(conflictIDs.Slice(), utxo.TransactionID.Base58),
				},
			}
			visualizerWorkerPool.TrySubmit(wsBlk)
			storeWsBlock(wsBlk)
		})
	})

	blkConfirmedClosure := event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
		blockID := event.Block.ID()
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetadata *tangleold.BlockMetadata) {
			wsBlk := &wsBlock{
				Type: BlkTypeTangleConfirmed,
				Data: &tangleConfirmed{
					ID:                    blockID.Base58(),
					ConfirmationState:     blkMetadata.ConfirmationState().String(),
					ConfirmationStateTime: blkMetadata.ConfirmationStateTime().UnixNano(),
				},
			}
			visualizerWorkerPool.TrySubmit(wsBlk)
			storeWsBlock(wsBlk)
		})
	})

	txAcceptedClosure := event.NewClosure(func(event *ledger.TransactionAcceptedEvent) {
		var blkID tangleold.BlockID
		deps.Tangle.Storage.Attachments(event.TransactionID).Consume(func(a *tangleold.Attachment) {
			blkID = a.BlockID()
		})

		wsBlk := &wsBlock{
			Type: BlkTypeTangleTxConfirmationState,
			Data: &tangleTxConfirmationStateChanged{
				ID:          blkID.Base58(),
				IsConfirmed: deps.AcceptanceGadget.IsTransactionConfirmed(event.TransactionID),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	deps.Tangle.Storage.Events.BlockStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.BlockBooked.Attach(bookedClosure)
	deps.AcceptanceGadget.Events().BlockAccepted.Attach(blkConfirmedClosure)
	deps.Tangle.Ledger.Events.TransactionAccepted.Attach(txAcceptedClosure)
}

func registerUTXOEvents() {
	storeClosure := event.NewClosure(func(event *tangleold.BlockStoredEvent) {
		if event.Block.Payload().Type() == devnetvm.TransactionType {
			tx := event.Block.Payload().(*devnetvm.Transaction)
			wsBlk := &wsBlock{
				Type: BlkTypeUTXOVertex,
				Data: newUTXOVertex(event.Block.ID(), tx),
			}
			visualizerWorkerPool.TrySubmit(wsBlk)
			storeWsBlock(wsBlk)
		}
	})

	bookedClosure := event.NewClosure(func(event *tangleold.BlockBookedEvent) {
		blockID := event.BlockID
		deps.Tangle.Storage.Block(blockID).Consume(func(block *tangleold.Block) {
			if block.Payload().Type() == devnetvm.TransactionType {
				tx := block.Payload().(*devnetvm.Transaction)
				deps.Tangle.Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
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
	})

	txAcceptedClosure := event.NewClosure(func(event *ledger.TransactionAcceptedEvent) {
		txID := event.TransactionID
		deps.Tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *ledger.TransactionMetadata) {
			wsBlk := &wsBlock{
				Type: BlkTypeUTXOConfirmationStateChanged,
				Data: &utxoConfirmationStateChanged{
					ID:                    txID.Base58(),
					ConfirmationState:     txMetadata.ConfirmationState().String(),
					ConfirmationStateTime: txMetadata.ConfirmationStateTime().UnixNano(),
					IsConfirmed:           deps.AcceptanceGadget.IsTransactionConfirmed(txID),
				},
			}
			visualizerWorkerPool.TrySubmit(wsBlk)
			storeWsBlock(wsBlk)
		})
	})

	deps.Tangle.Storage.Events.BlockStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.BlockBooked.Attach(bookedClosure)
	deps.Tangle.Ledger.Events.TransactionAccepted.Attach(txAcceptedClosure)
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

	conflictConfirmedClosure := event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		wsBlk := &wsBlock{
			Type: BlkTypeConflictConfirmationStateChanged,
			Data: &conflictConfirmationStateChanged{
				ID:                event.ID.Base58(),
				ConfirmationState: confirmation.Accepted.String(),
				IsConfirmed:       true,
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	conflictWeightChangedClosure := event.NewClosure(func(e *tangleold.ConflictWeightChangedEvent) {
		conflictConfirmationState := deps.Tangle.Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(e.ConflictID))
		wsBlk := &wsBlock{
			Type: BlkTypeConflictWeightChanged,
			Data: &conflictWeightChanged{
				ID:                e.ConflictID.Base58(),
				Weight:            e.Weight,
				ConfirmationState: conflictConfirmationState.String(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(createdClosure)
	deps.Tangle.Ledger.ConflictDAG.Events.ConflictAccepted.Attach(conflictConfirmedClosure)
	deps.Tangle.Ledger.ConflictDAG.Events.ConflictParentsUpdated.Attach(parentUpdateClosure)
	deps.Tangle.ApprovalWeightManager.Events.ConflictWeightChanged.Attach(conflictWeightChangedClosure)
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

		blocks := []*tangleVertex{}
		txs := []*utxoVertex{}
		conflicts := []*conflictVertex{}
		conflictMap := set.NewAdvancedSet[utxo.TransactionID]()
		entryBlks := tangleold.NewBlockIDs()
		deps.Tangle.Storage.Children(tangleold.EmptyBlockID).Consume(func(child *tangleold.Child) {
			entryBlks.Add(child.ChildBlockID())
		})

		deps.Tangle.Utils.WalkBlockID(func(blockID tangleold.BlockID, walker *walker.Walker[tangleold.BlockID]) {
			deps.Tangle.Storage.Block(blockID).Consume(func(blk *tangleold.Block) {
				// only keep blocks that is issued in the given time interval
				if blk.IssuingTime().After(startTimestamp) && blk.IssuingTime().Before(endTimestamp) {
					// add block
					tangleNode := newTangleVertex(blk)
					blocks = append(blocks, tangleNode)

					// add tx
					if tangleNode.IsTx {
						utxoNode := newUTXOVertex(blk.ID(), blk.Payload().(*devnetvm.Transaction))
						txs = append(txs, utxoNode)
					}

					// add conflict
					conflictIDs, err := deps.Tangle.Booker.BlockConflictIDs(blk.ID())
					if err != nil {
						conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
					}
					for it := conflictIDs.Iterator(); it.HasNext(); {
						conflictID := it.Next()
						if conflictMap.Has(conflictID) {
							continue
						}

						conflictMap.Add(conflictID)
						conflicts = append(conflicts, newConflictVertex(conflictID))
					}
				}

				// continue walking if the block is issued before endTimestamp
				if blk.IssuingTime().Before(endTimestamp) {
					deps.Tangle.Storage.Children(blockID).Consume(func(child *tangleold.Child) {
						walker.Push(child.ChildBlockID())
					})
				}
			})
		}, entryBlks)

		return c.JSON(http.StatusOK, searchResult{Blocks: blocks, Txs: txs, Conflicts: conflicts})
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

func newTangleVertex(block *tangleold.Block) (ret *tangleVertex) {
	deps.Tangle.Storage.BlockMetadata(block.ID()).Consume(func(blkMetadata *tangleold.BlockMetadata) {
		conflictIDs, err := deps.Tangle.Booker.BlockConflictIDs(block.ID())
		if err != nil {
			conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
		}
		ret = &tangleVertex{
			ID:                    block.ID().Base58(),
			StrongParentIDs:       block.ParentsByType(tangleold.StrongParentType).Base58(),
			WeakParentIDs:         block.ParentsByType(tangleold.WeakParentType).Base58(),
			ShallowLikeParentIDs:  block.ParentsByType(tangleold.ShallowLikeParentType).Base58(),
			ConflictIDs:           lo.Map(conflictIDs.Slice(), utxo.TransactionID.Base58),
			IsMarker:              blkMetadata.StructureDetails() != nil && blkMetadata.StructureDetails().IsPastMarker(),
			IsTx:                  block.Payload().Type() == devnetvm.TransactionType,
			IsConfirmed:           deps.AcceptanceGadget.IsBlockConfirmed(block.ID()),
			ConfirmationStateTime: blkMetadata.ConfirmationStateTime().UnixNano(),
			ConfirmationState:     blkMetadata.ConfirmationState().String(),
		}
	})

	if ret.IsTx {
		ret.TxID = block.Payload().(*devnetvm.Transaction).ID().Base58()
	}
	return
}

func newUTXOVertex(blkID tangleold.BlockID, tx *devnetvm.Transaction) (ret *utxoVertex) {
	inputs := make([]*jsonmodels.Input, len(tx.Essence().Inputs()))
	for i, input := range tx.Essence().Inputs() {
		inputs[i] = jsonmodels.NewInput(input)
	}

	outputs := make([]string, len(tx.Essence().Outputs()))
	for i, output := range tx.Essence().Outputs() {
		outputs[i] = output.ID().Base58()
	}

	var confirmationState string
	var confirmedTime int64
	var conflictIDs []string
	deps.Tangle.Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
		confirmationState = txMetadata.ConfirmationState().String()
		confirmedTime = txMetadata.ConfirmationStateTime().UnixNano()
		conflictIDs = lo.Map(txMetadata.ConflictIDs().Slice(), utxo.TransactionID.Base58)
	})

	ret = &utxoVertex{
		BlkID:                 blkID.Base58(),
		ID:                    tx.ID().Base58(),
		Inputs:                inputs,
		Outputs:               outputs,
		IsConfirmed:           deps.AcceptanceGadget.IsTransactionConfirmed(tx.ID()),
		ConflictIDs:           conflictIDs,
		ConfirmationState:     confirmationState,
		ConfirmationStateTime: confirmedTime,
	}

	return ret
}

func newConflictVertex(conflictID utxo.TransactionID) (ret *conflictVertex) {
	deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflict(conflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		conflicts := make(map[utxo.OutputID][]utxo.TransactionID)
		// get conflicts of a conflict
		for it := conflict.ConflictSetIDs().Iterator(); it.HasNext(); {
			conflictID := it.Next()
			conflicts[conflictID] = make([]utxo.TransactionID, 0)
			deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflictMembers(conflictID).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {
				conflicts[conflictID] = append(conflicts[conflictID], conflictMember.ConflictID())
			})
		}

		ret = &conflictVertex{
			ID:                conflictID.Base58(),
			Parents:           lo.Map(conflict.Parents().Slice(), utxo.TransactionID.Base58),
			Conflicts:         jsonmodels.NewGetConflictConflictsResponse(conflict.ID(), conflicts),
			IsConfirmed:       deps.AcceptanceGadget.IsConflictConfirmed(conflictID),
			ConfirmationState: deps.Tangle.Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(conflictID)).String(),
			AW:                deps.Tangle.ApprovalWeightManager.WeightOfConflict(conflictID),
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
