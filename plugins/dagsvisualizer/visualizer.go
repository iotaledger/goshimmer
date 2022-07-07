package dagsvisualizer

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/types/confirmation"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
		registerBranchEvents()

		<-ctx.Done()
		log.Info("Stopping DAGs Visualizer ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping DAGs Visualizer ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerTangleEvents() {
	storeClosure := event.NewClosure(func(event *tangle.BlockStoredEvent) {
		wsBlk := &wsBlock{
			Type: BlkTypeTangleVertex,
			Data: newTangleVertex(event.Block),
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	bookedClosure := event.NewClosure(func(event *tangle.BlockBookedEvent) {
		blockID := event.BlockID
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetadata *tangle.BlockMetadata) {
			branchIDs, err := deps.Tangle.Booker.BlockBranchIDs(blockID)
			if err != nil {
				branchIDs = set.NewAdvancedSet[utxo.TransactionID]()
			}

			wsBlk := &wsBlock{
				Type: BlkTypeTangleBooked,
				Data: &tangleBooked{
					ID:        blockID.Base58(),
					IsMarker:  blkMetadata.StructureDetails().IsPastMarker(),
					BranchIDs: lo.Map(branchIDs.Slice(), utxo.TransactionID.Base58),
				},
			}
			visualizerWorkerPool.TrySubmit(wsBlk)
			storeWsBlock(wsBlk)
		})
	})

	blkConfirmedClosure := event.NewClosure(func(event *tangle.BlockAcceptedEvent) {
		blockID := event.Block.ID()
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetadata *tangle.BlockMetadata) {
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
		var blkID tangle.BlockID
		deps.Tangle.Storage.Attachments(event.TransactionID).Consume(func(a *tangle.Attachment) {
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
	storeClosure := event.NewClosure(func(event *tangle.BlockStoredEvent) {
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

	bookedClosure := event.NewClosure(func(event *tangle.BlockBookedEvent) {
		blockID := event.BlockID
		deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
			if block.Payload().Type() == devnetvm.TransactionType {
				tx := block.Payload().(*devnetvm.Transaction)
				deps.Tangle.Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
					wsBlk := &wsBlock{
						Type: BlkTypeUTXOBooked,
						Data: &utxoBooked{
							ID:        tx.ID().Base58(),
							BranchIDs: lo.Map(txMetadata.BranchIDs().Slice(), utxo.TransactionID.Base58),
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

func registerBranchEvents() {
	createdClosure := event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		wsBlk := &wsBlock{
			Type: BlkTypeBranchVertex,
			Data: newBranchVertex(event.ID),
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	parentUpdateClosure := event.NewClosure(func(event *conflictdag.BranchParentsUpdatedEvent[utxo.TransactionID, utxo.OutputID]) {
		lo.Map(event.ParentsBranchIDs.Slice(), utxo.TransactionID.Base58)
		wsBlk := &wsBlock{
			Type: BlkTypeBranchParentsUpdate,
			Data: &branchParentUpdate{
				ID:      event.BranchID.Base58(),
				Parents: lo.Map(event.ParentsBranchIDs.Slice(), utxo.TransactionID.Base58),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	branchConfirmedClosure := event.NewClosure(func(event *conflictdag.BranchAcceptedEvent[utxo.TransactionID]) {
		wsBlk := &wsBlock{
			Type: BlkTypeBranchConfirmationStateChanged,
			Data: &branchConfirmationStateChanged{
				ID:                event.ID.Base58(),
				ConfirmationState: confirmation.Accepted.String(),
				IsConfirmed:       true,
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	branchWeightChangedClosure := event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		branchConfirmationState := deps.Tangle.Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(e.BranchID))
		wsBlk := &wsBlock{
			Type: BlkTypeBranchWeightChanged,
			Data: &branchWeightChanged{
				ID:                e.BranchID.Base58(),
				Weight:            e.Weight,
				ConfirmationState: branchConfirmationState.String(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsBlk)
		storeWsBlock(wsBlk)
	})

	deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(createdClosure)
	deps.Tangle.Ledger.ConflictDAG.Events.BranchAccepted.Attach(branchConfirmedClosure)
	deps.Tangle.Ledger.ConflictDAG.Events.BranchParentsUpdated.Attach(parentUpdateClosure)
	deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.Attach(branchWeightChangedClosure)
}

func setupDagsVisualizerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/dagsvisualizer/branch/:branchID", func(c echo.Context) (err error) {
		parents := make(map[string]*branchVertex)
		var branchID utxo.TransactionID
		if err = branchID.FromBase58(c.Param("branchID")); err != nil {
			err = c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
			return
		}
		vertex := newBranchVertex(branchID)
		parents[vertex.ID] = vertex
		getBranchesToMaster(vertex, parents)

		var branches []*branchVertex
		for _, branch := range parents {
			branches = append(branches, branch)
		}
		return c.JSON(http.StatusOK, branches)
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
		branches := []*branchVertex{}
		branchMap := set.NewAdvancedSet[utxo.TransactionID]()
		entryBlks := tangle.NewBlockIDs()
		deps.Tangle.Storage.Childs(tangle.EmptyBlockID).Consume(func(child *tangle.Child) {
			entryBlks.Add(child.ChildBlockID())
		})

		deps.Tangle.Utils.WalkBlockID(func(blockID tangle.BlockID, walker *walker.Walker[tangle.BlockID]) {
			deps.Tangle.Storage.Block(blockID).Consume(func(blk *tangle.Block) {
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

					// add branch
					branchIDs, err := deps.Tangle.Booker.BlockBranchIDs(blk.ID())
					if err != nil {
						branchIDs = set.NewAdvancedSet[utxo.TransactionID]()
					}
					for it := branchIDs.Iterator(); it.HasNext(); {
						branchID := it.Next()
						if branchMap.Has(branchID) {
							continue
						}

						branchMap.Add(branchID)
						branches = append(branches, newBranchVertex(branchID))
					}
				}

				// continue walking if the block is issued before endTimestamp
				if blk.IssuingTime().Before(endTimestamp) {
					deps.Tangle.Storage.Childs(blockID).Consume(func(child *tangle.Child) {
						walker.Push(child.ChildBlockID())
					})
				}
			})
		}, entryBlks)

		return c.JSON(http.StatusOK, searchResult{Blocks: blocks, Txs: txs, Branches: branches})
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

func newTangleVertex(block *tangle.Block) (ret *tangleVertex) {
	deps.Tangle.Storage.BlockMetadata(block.ID()).Consume(func(blkMetadata *tangle.BlockMetadata) {
		branchIDs, err := deps.Tangle.Booker.BlockBranchIDs(block.ID())
		if err != nil {
			branchIDs = set.NewAdvancedSet[utxo.TransactionID]()
		}
		ret = &tangleVertex{
			ID:                    block.ID().Base58(),
			StrongParentIDs:       block.ParentsByType(tangle.StrongParentType).Base58(),
			WeakParentIDs:         block.ParentsByType(tangle.WeakParentType).Base58(),
			ShallowLikeParentIDs:  block.ParentsByType(tangle.ShallowLikeParentType).Base58(),
			BranchIDs:             lo.Map(branchIDs.Slice(), utxo.TransactionID.Base58),
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

func newUTXOVertex(blkID tangle.BlockID, tx *devnetvm.Transaction) (ret *utxoVertex) {
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
	var branchIDs []string
	deps.Tangle.Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
		confirmationState = txMetadata.ConfirmationState().String()
		confirmedTime = txMetadata.ConfirmationStateTime().UnixNano()
		branchIDs = lo.Map(txMetadata.BranchIDs().Slice(), utxo.TransactionID.Base58)
	})

	ret = &utxoVertex{
		BlkID:                 blkID.Base58(),
		ID:                    tx.ID().Base58(),
		Inputs:                inputs,
		Outputs:               outputs,
		IsConfirmed:           deps.AcceptanceGadget.IsTransactionConfirmed(tx.ID()),
		BranchIDs:             branchIDs,
		ConfirmationState:     confirmationState,
		ConfirmationStateTime: confirmedTime,
	}

	return ret
}

func newBranchVertex(branchID utxo.TransactionID) (ret *branchVertex) {
	deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflict(branchID).Consume(func(branch *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		conflicts := make(map[utxo.OutputID][]utxo.TransactionID)
		// get conflicts of a branch
		for it := branch.ConflictIDs().Iterator(); it.HasNext(); {
			conflictID := it.Next()
			conflicts[conflictID] = make([]utxo.TransactionID, 0)
			deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflictMembers(conflictID).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {
				conflicts[conflictID] = append(conflicts[conflictID], conflictMember.ConflictID())
			})
		}

		ret = &branchVertex{
			ID:                branchID.Base58(),
			Parents:           lo.Map(branch.Parents().Slice(), utxo.TransactionID.Base58),
			Conflicts:         jsonmodels.NewGetBranchConflictsResponse(branch.ID(), conflicts),
			IsConfirmed:       deps.AcceptanceGadget.IsBranchConfirmed(branchID),
			ConfirmationState: deps.Tangle.Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(branchID)).String(),
			AW:                deps.Tangle.ApprovalWeightManager.WeightOfBranch(branchID),
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

func getBranchesToMaster(vertex *branchVertex, parents map[string]*branchVertex) {
	for _, IDBase58 := range vertex.Parents {
		if _, ok := parents[IDBase58]; !ok {
			var ID utxo.TransactionID
			if err := ID.FromBase58(IDBase58); err == nil {
				parentVertex := newBranchVertex(ID)
				parents[parentVertex.ID] = parentVertex
				getBranchesToMaster(parentVertex, parents)
			}
		}
	}
}
