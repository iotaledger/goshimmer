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
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
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
	maxWsMessageBufferSize    = 200
	buffer                    []*wsMessage
	bufferMutex               sync.RWMutex
)

func setupVisualizer() {
	// create and start workerpool
	visualizerWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		broadcastWsMessage(task.Param(0))
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
	storeClosure := event.NewClosure(func(event *tangle.MessageStoredEvent) {
		wsMsg := &wsMessage{
			Type: MsgTypeTangleVertex,
			Data: newTangleVertex(event.Message),
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	bookedClosure := event.NewClosure(func(event *tangle.MessageBookedEvent) {
		messageID := event.MessageID
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
			branchIDs, err := deps.Tangle.Booker.MessageBranchIDs(messageID)
			if err != nil {
				branchIDs = set.NewAdvancedSet[utxo.TransactionID]()
			}

			wsMsg := &wsMessage{
				Type: MsgTypeTangleBooked,
				Data: &tangleBooked{
					ID:        messageID.Base58(),
					IsMarker:  msgMetadata.StructureDetails().IsPastMarker(),
					BranchIDs: lo.Map(branchIDs.Slice(), utxo.TransactionID.Base58),
				},
			}
			visualizerWorkerPool.TrySubmit(wsMsg)
			storeWsMessage(wsMsg)
		})
	})

	msgConfirmedClosure := event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		messageID := event.Message.ID()
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
			wsMsg := &wsMessage{
				Type: MsgTypeTangleConfirmed,
				Data: &tangleConfirmed{
					ID:            messageID.Base58(),
					GoF:           msgMetadata.GradeOfFinality().String(),
					ConfirmedTime: msgMetadata.GradeOfFinalityTime().UnixNano(),
				},
			}
			visualizerWorkerPool.TrySubmit(wsMsg)
			storeWsMessage(wsMsg)
		})
	})

	txGoFChangedClosure := event.NewClosure(func(event *ledger.TransactionConfirmedEvent) {
		var msgID tangle.MessageID
		deps.Tangle.Storage.Attachments(event.TransactionID).Consume(func(a *tangle.Attachment) {
			msgID = a.MessageID()
		})

		wsMsg := &wsMessage{
			Type: MsgTypeTangleTxGoF,
			Data: &tangleTxGoFChanged{
				ID:          msgID.Base58(),
				IsConfirmed: deps.FinalityGadget.IsTransactionConfirmed(event.TransactionID),
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	deps.Tangle.Storage.Events.MessageStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.MessageBooked.Attach(bookedClosure)
	deps.FinalityGadget.Events().MessageConfirmed.Attach(msgConfirmedClosure)
	deps.Tangle.Ledger.Events.TransactionConfirmed.Attach(txGoFChangedClosure)
}

func registerUTXOEvents() {
	storeClosure := event.NewClosure(func(event *tangle.MessageStoredEvent) {
		if event.Message.Payload().Type() == devnetvm.TransactionType {
			tx := event.Message.Payload().(*devnetvm.Transaction)
			wsMsg := &wsMessage{
				Type: MsgTypeUTXOVertex,
				Data: newUTXOVertex(event.Message.ID(), tx),
			}
			visualizerWorkerPool.TrySubmit(wsMsg)
			storeWsMessage(wsMsg)
		}
	})

	bookedClosure := event.NewClosure(func(event *tangle.MessageBookedEvent) {
		messageID := event.MessageID
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == devnetvm.TransactionType {
				tx := message.Payload().(*devnetvm.Transaction)
				deps.Tangle.Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
					wsMsg := &wsMessage{
						Type: MsgTypeUTXOBooked,
						Data: &utxoBooked{
							ID:        tx.ID().Base58(),
							BranchIDs: lo.Map(txMetadata.BranchIDs().Slice(), utxo.TransactionID.Base58),
						},
					}
					visualizerWorkerPool.TrySubmit(wsMsg)
					storeWsMessage(wsMsg)
				})
			}
		})
	})

	txGoFChangedClosure := event.NewClosure(func(event *ledger.TransactionConfirmedEvent) {
		txID := event.TransactionID
		deps.Tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *ledger.TransactionMetadata) {
			wsMsg := &wsMessage{
				Type: MsgTypeUTXOGoFChanged,
				Data: &utxoGoFChanged{
					ID:          txID.Base58(),
					GoF:         txMetadata.GradeOfFinality().String(),
					GoFTime:     txMetadata.GradeOfFinalityTime().UnixNano(),
					IsConfirmed: deps.FinalityGadget.IsTransactionConfirmed(txID),
				},
			}
			visualizerWorkerPool.TrySubmit(wsMsg)
			storeWsMessage(wsMsg)
		})
	})

	deps.Tangle.Storage.Events.MessageStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.MessageBooked.Attach(bookedClosure)
	deps.Tangle.Ledger.Events.TransactionConfirmed.Attach(txGoFChangedClosure)
}

func registerBranchEvents() {
	createdClosure := event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		wsMsg := &wsMessage{
			Type: MsgTypeBranchVertex,
			Data: newBranchVertex(event.ID),
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	parentUpdateClosure := event.NewClosure(func(event *conflictdag.BranchParentsUpdatedEvent[utxo.TransactionID, utxo.OutputID]) {
		lo.Map(event.ParentsBranchIDs.Slice(), utxo.TransactionID.Base58)
		wsMsg := &wsMessage{
			Type: MsgTypeBranchParentsUpdate,
			Data: &branchParentUpdate{
				ID:      event.BranchID.Base58(),
				Parents: lo.Map(event.ParentsBranchIDs.Slice(), utxo.TransactionID.Base58),
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	branchConfirmedClosure := event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		wsMsg := &wsMessage{
			Type: MsgTypeBranchGoFChanged,
			Data: &branchGoFChanged{
				ID:          event.BranchID.Base58(),
				GoF:         gof.High.String(),
				IsConfirmed: true,
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	branchWeightChangedClosure := event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		branchGoF, _ := deps.Tangle.Ledger.Utils.BranchGradeOfFinality(e.BranchID)
		wsMsg := &wsMessage{
			Type: MsgTypeBranchWeightChanged,
			Data: &branchWeightChanged{
				ID:     e.BranchID.Base58(),
				Weight: e.Weight,
				GoF:    branchGoF.String(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(createdClosure)
	deps.Tangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(branchConfirmedClosure)
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

		messages := []*tangleVertex{}
		txs := []*utxoVertex{}
		branches := []*branchVertex{}
		branchMap := set.NewAdvancedSet[utxo.TransactionID]()
		entryMsgs := tangle.NewMessageIDs()
		deps.Tangle.Storage.Approvers(tangle.EmptyMessageID).Consume(func(approver *tangle.Approver) {
			entryMsgs.Add(approver.ApproverMessageID())
		})

		deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker[tangle.MessageID]) {
			deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
				// only keep messages that is issued in the given time interval
				if msg.IssuingTime().After(startTimestamp) && msg.IssuingTime().Before(endTimestamp) {
					// add message
					tangleNode := newTangleVertex(msg)
					messages = append(messages, tangleNode)

					// add tx
					if tangleNode.IsTx {
						utxoNode := newUTXOVertex(msg.ID(), msg.Payload().(*devnetvm.Transaction))
						txs = append(txs, utxoNode)
					}

					// add branch
					branchIDs, err := deps.Tangle.Booker.MessageBranchIDs(msg.ID())
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

				// continue walking if the message is issued before endTimestamp
				if msg.IssuingTime().Before(endTimestamp) {
					deps.Tangle.Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
						walker.Push(approver.ApproverMessageID())
					})
				}
			})
		}, entryMsgs)

		return c.JSON(http.StatusOK, searchResult{Messages: messages, Txs: txs, Branches: branches})
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

func newTangleVertex(message *tangle.Message) (ret *tangleVertex) {
	deps.Tangle.Storage.MessageMetadata(message.ID()).Consume(func(msgMetadata *tangle.MessageMetadata) {
		branchIDs, err := deps.Tangle.Booker.MessageBranchIDs(message.ID())
		if err != nil {
			branchIDs = set.NewAdvancedSet[utxo.TransactionID]()
		}
		ret = &tangleVertex{
			ID:                   message.ID().Base58(),
			StrongParentIDs:      message.ParentsByType(tangle.StrongParentType).Base58(),
			WeakParentIDs:        message.ParentsByType(tangle.WeakParentType).Base58(),
			ShallowLikeParentIDs: message.ParentsByType(tangle.ShallowLikeParentType).Base58(),
			BranchIDs:            lo.Map(branchIDs.Slice(), utxo.TransactionID.Base58),
			IsMarker:             msgMetadata.StructureDetails() != nil && msgMetadata.StructureDetails().IsPastMarker(),
			IsTx:                 message.Payload().Type() == devnetvm.TransactionType,
			IsConfirmed:          deps.FinalityGadget.IsMessageConfirmed(message.ID()),
			ConfirmedTime:        msgMetadata.GradeOfFinalityTime().UnixNano(),
			GoF:                  msgMetadata.GradeOfFinality().String(),
		}
	})

	if ret.IsTx {
		ret.TxID = message.Payload().(*devnetvm.Transaction).ID().Base58()
	}
	return
}

func newUTXOVertex(msgID tangle.MessageID, tx *devnetvm.Transaction) (ret *utxoVertex) {
	inputs := make([]*jsonmodels.Input, len(tx.Essence().Inputs()))
	for i, input := range tx.Essence().Inputs() {
		inputs[i] = jsonmodels.NewInput(input)
	}

	outputs := make([]string, len(tx.Essence().Outputs()))
	for i, output := range tx.Essence().Outputs() {
		outputs[i] = output.ID().Base58()
	}

	var gof string
	var confirmedTime int64
	var branchIDs []string
	deps.Tangle.Ledger.Storage.CachedTransactionMetadata(tx.ID()).Consume(func(txMetadata *ledger.TransactionMetadata) {
		gof = txMetadata.GradeOfFinality().String()
		confirmedTime = txMetadata.GradeOfFinalityTime().UnixNano()
		branchIDs = lo.Map(txMetadata.BranchIDs().Slice(), utxo.TransactionID.Base58)
	})

	ret = &utxoVertex{
		MsgID:       msgID.Base58(),
		ID:          tx.ID().Base58(),
		Inputs:      inputs,
		Outputs:     outputs,
		IsConfirmed: deps.FinalityGadget.IsTransactionConfirmed(tx.ID()),
		BranchIDs:   branchIDs,
		GoF:         gof,
		GoFTime:     confirmedTime,
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

		branchGoF, _ := deps.Tangle.Ledger.Utils.BranchGradeOfFinality(branchID)
		ret = &branchVertex{
			ID:          branchID.Base58(),
			Parents:     lo.Map(branch.Parents().Slice(), utxo.TransactionID.Base58),
			Conflicts:   jsonmodels.NewGetBranchConflictsResponse(branch.ID(), conflicts),
			IsConfirmed: deps.FinalityGadget.IsBranchConfirmed(branchID),
			GoF:         branchGoF.String(),
			AW:          deps.Tangle.ApprovalWeightManager.WeightOfBranch(branchID),
		}
	})
	return
}

func storeWsMessage(msg *wsMessage) {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()
	if len(buffer) >= maxWsMessageBufferSize {
		buffer = buffer[1:]
	}
	buffer = append(buffer, msg)
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
