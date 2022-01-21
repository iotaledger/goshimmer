package dagsvisualizer

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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
	storeClosure := events.NewClosure(func(messageID tangle.MessageID) {
		wsMsg := &wsMessage{
			Type: MsgTypeTangleVertex,
			Data: newTangleVertex(messageID),
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	bookedClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
			branchID, err := deps.Tangle.Booker.MessageBranchID(messageID)
			if err != nil {
				branchID = ledgerstate.BranchID{}
			}

			wsMsg := &wsMessage{
				Type: MsgTypeTangleBooked,
				Data: &tangleBooked{
					ID:       messageID.Base58(),
					IsMarker: msgMetadata.StructureDetails().IsPastMarker,
					BranchID: branchID.Base58(),
				},
			}
			visualizerWorkerPool.TrySubmit(wsMsg)
			storeWsMessage(wsMsg)
		})
	})

	msgConfirmedClosure := events.NewClosure(func(messageID tangle.MessageID) {
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

	fmUpdateClosure := events.NewClosure(func(fmUpdate *tangle.FutureMarkerUpdate) {
		wsMsg := &wsMessage{
			Type: MsgTypeFutureMarkerUpdated,
			Data: &tangleFutureMarkerUpdated{
				ID:             fmUpdate.ID.Base58(),
				FutureMarkerID: fmUpdate.FutureMarker.Base58(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	deps.Tangle.Storage.Events.MessageStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.MessageBooked.Attach(bookedClosure)
	deps.Tangle.Booker.MarkersManager.Events.FutureMarkerUpdated.Attach(fmUpdateClosure)
	deps.FinalityGadget.Events().MessageConfirmed.Attach(msgConfirmedClosure)
}

func registerUTXOEvents() {
	storeClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			if msg.Payload().Type() == ledgerstate.TransactionType {
				tx := msg.Payload().(*ledgerstate.Transaction)
				wsMsg := &wsMessage{
					Type: MsgTypeUTXOVertex,
					Data: newUTXOVertex(messageID, tx),
				}
				visualizerWorkerPool.TrySubmit(wsMsg)
				storeWsMessage(wsMsg)
			}
		})
	})

	bookedClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == ledgerstate.TransactionType {
				branchID, err := deps.Tangle.Booker.MessageBranchID(messageID)
				if err != nil {
					branchID = ledgerstate.BranchID{}
				}
				wsMsg := &wsMessage{
					Type: MsgTypeUTXOBooked,
					Data: &utxoBooked{
						ID:       message.Payload().(*ledgerstate.Transaction).ID().Base58(),
						BranchID: branchID.Base58(),
					},
				}
				visualizerWorkerPool.TrySubmit(wsMsg)
				storeWsMessage(wsMsg)
			}
		})
	})

	txConfirmedClosure := events.NewClosure(func(txID ledgerstate.TransactionID) {
		deps.Tangle.LedgerState.TransactionMetadata(txID).Consume(func(txMetadata *ledgerstate.TransactionMetadata) {
			wsMsg := &wsMessage{
				Type: MsgTypeUTXOConfirmed,
				Data: &utxoConfirmed{
					ID:            txID.Base58(),
					GoF:           txMetadata.GradeOfFinality().String(),
					ConfirmedTime: txMetadata.GradeOfFinalityTime().UnixNano(),
				},
			}
			visualizerWorkerPool.TrySubmit(wsMsg)
			storeWsMessage(wsMsg)
		})
	})

	deps.Tangle.Storage.Events.MessageStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.MessageBooked.Attach(bookedClosure)
	deps.FinalityGadget.Events().TransactionConfirmed.Attach(txConfirmedClosure)
}

func registerBranchEvents() {
	createdClosure := events.NewClosure(func(branchID ledgerstate.BranchID) {
		wsMsg := &wsMessage{
			Type: MsgTypeBranchVertex,
			Data: newBranchVertex(branchID),
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	parentUpdateClosure := events.NewClosure(func(parentUpdate *ledgerstate.BranchParentUpdate) {
		wsMsg := &wsMessage{
			Type: MsgTypeBranchParentsUpdate,
			Data: &branchParentUpdate{
				ID:      parentUpdate.ID.Base58(),
				Parents: parentUpdate.NewParents.Strings(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	branchConfirmedClosure := events.NewClosure(func(branchID ledgerstate.BranchID) {
		wsMsg := &wsMessage{
			Type: MsgTypeBranchConfirmed,
			Data: &branchConfirmed{
				ID: branchID.Base58(),
			},
		}
		visualizerWorkerPool.TrySubmit(wsMsg)
		storeWsMessage(wsMsg)
	})

	branchWeightChangedClosure := events.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		branchGoF, _ := deps.Tangle.LedgerState.UTXODAG.BranchGradeOfFinality(e.BranchID)
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

	deps.Tangle.LedgerState.BranchDAG.Events.BranchCreated.Attach(createdClosure)
	deps.FinalityGadget.Events().BranchConfirmed.Attach(branchConfirmedClosure)
	deps.Tangle.LedgerState.BranchDAG.Events.BranchParentsUpdated.Attach(parentUpdateClosure)
	deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.Attach(branchWeightChangedClosure)
}

func setupDagsVisualizerRoutes(routeGroup *echo.Group) {
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
		branchMap := make(map[ledgerstate.BranchID]struct{})

		deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
			deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
				// only keep messages that is issued in the given time interval
				if msg.IssuingTime().After(startTimestamp) && msg.IssuingTime().Before(endTimestamp) {
					// add message
					tangleNode := newTangleVertex(msg.ID())
					messages = append(messages, tangleNode)

					// add tx
					if tangleNode.IsTx {
						utxoNode := newUTXOVertex(msg.ID(), msg.Payload().(*ledgerstate.Transaction))
						txs = append(txs, utxoNode)
					}

					// add branch
					branchID, err := deps.Tangle.Booker.MessageBranchID(msg.ID())
					if err != nil {
						branchID = ledgerstate.BranchID{}
					}
					if _, ok := branchMap[branchID]; !ok {
						branchMap[branchID] = struct{}{}

						branchNode := newBranchVertex(branchID)
						branches = append(branches, branchNode)
					}
				}
			})

			deps.Tangle.Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
				walker.Push(approver.ApproverMessageID())
			})
		}, tangle.MessageIDs{tangle.EmptyMessageID})

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

func newTangleVertex(messageID tangle.MessageID) (ret *tangleVertex) {
	deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
			ret = &tangleVertex{
				ID:              messageID.Base58(),
				StrongParentIDs: msg.ParentsByType(tangle.StrongParentType).ToStrings(),
				WeakParentIDs:   msg.ParentsByType(tangle.WeakParentType).ToStrings(),
				LikedParentIDs:  msg.ParentsByType(tangle.LikeParentType).ToStrings(),
				BranchID:        msgMetadata.BranchID().Base58(),
				IsMarker:        msgMetadata.StructureDetails() != nil && msgMetadata.StructureDetails().IsPastMarker,
				IsTx:            msg.Payload().Type() == ledgerstate.TransactionType,
				IsConfirmed:     deps.FinalityGadget.IsMessageConfirmed(messageID),
				ConfirmedTime:   msgMetadata.GradeOfFinalityTime().UnixNano(),
				GoF:             msgMetadata.GradeOfFinality().String(),
			}
		})

		if ret.IsTx {
			ret.TxID = msg.Payload().(*ledgerstate.Transaction).ID().Base58()
		}
	})
	return
}

func newUTXOVertex(msgID tangle.MessageID, tx *ledgerstate.Transaction) (ret *utxoVertex) {
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
	deps.Tangle.LedgerState.TransactionMetadata(tx.ID()).Consume(func(txMetadata *ledgerstate.TransactionMetadata) {
		gof = txMetadata.GradeOfFinality().String()
		confirmedTime = txMetadata.GradeOfFinalityTime().UnixNano()
	})

	ret = &utxoVertex{
		MsgID:         msgID.Base58(),
		ID:            tx.ID().Base58(),
		Inputs:        inputs,
		Outputs:       outputs,
		IsConfirmed:   deps.FinalityGadget.IsTransactionConfirmed(tx.ID()),
		GoF:           gof,
		ConfirmedTime: confirmedTime,
	}

	return
}

func newBranchVertex(branchID ledgerstate.BranchID) (ret *branchVertex) {
	deps.Tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		conflicts := make(map[ledgerstate.ConflictID][]ledgerstate.BranchID)
		// get conflicts of a Conflict branch
		if branch.Type() == ledgerstate.ConflictBranchType {
			for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
				conflicts[conflictID] = make([]ledgerstate.BranchID, 0)
				deps.Tangle.LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
					conflicts[conflictID] = append(conflicts[conflictID], conflictMember.BranchID())
				})
			}
		}

		branchGoF, _ := deps.Tangle.LedgerState.UTXODAG.BranchGradeOfFinality(branchID)
		ret = &branchVertex{
			ID:          branchID.Base58(),
			Type:        branch.Type().String(),
			Parents:     branch.Parents().Strings(),
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
