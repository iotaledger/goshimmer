package dagsvisualizer

import (
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

func setupVisualizer() {
	// create and start workerpool
	visualizerWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		broadcastWsMessage(task.Param(0))
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))
}

func runVisualizer() {
	if err := daemon.BackgroundWorker("DAGs Visualizer", func(shutdownSignal <-chan struct{}) {
		// register to events
		registerTangleEvents()
		registerUTXOEvents()
		registerBranchEvents()

		<-shutdownSignal
		log.Info("Stopping DAGs Visualizer ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping DAGs Visualizer ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerTangleEvents() {
	storeClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			visualizerWorkerPool.TrySubmit(&wsMessage{
				Type: MsgTypeTangleVertex,
				Data: &tangleVertex{
					ID:              messageID.Base58(),
					StrongParentIDs: msg.StrongParents().ToStrings(),
					WeakParentIDs:   msg.WeakParents().ToStrings(),
					BranchID:        ledgerstate.UndefinedBranchID.Base58(),
					IsMarker:        false,
					IsTx:            msg.Payload().Type() == ledgerstate.TransactionType,
					ConfirmedTime:   0,
				},
			})
		})
	})

	bookedClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
			visualizerWorkerPool.TrySubmit((&wsMessage{
				Type: MsgTypeTangleBooked,
				Data: &tangleBooked{
					ID:       messageID.Base58(),
					IsMarker: msgMetadata.StructureDetails().IsPastMarker,
					BranchID: msgMetadata.BranchID().Base58(),
				},
			}))
		})
	})

	finalizedClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
			visualizerWorkerPool.TrySubmit(&wsMessage{
				Type: MsgTypeTangleConfirmed,
				Data: &tangleFinalized{
					ID:            messageID.Base58(),
					ConfirmedTime: msgMetadata.FinalizedTime().UnixNano(),
				},
			})
		})
	})

	fmUpdateClosure := events.NewClosure(func(fmUpdate *tangle.FutureMarkerUpdate) {
		visualizerWorkerPool.TrySubmit(&wsMessage{
			Type: MsgTypeFutureMarkerUpdated,
			Data: &tangleFutureMarkerUpdated{
				ID:             fmUpdate.ID.Base58(),
				FutureMarkerID: fmUpdate.FutureMarker.Base58(),
			},
		})
	})

	markerConfirmedClosure := events.NewClosure(func(markerAWUpdate *tangle.MarkerAWUpdated) {
		// get message ID of marker
		visualizerWorkerPool.TrySubmit(&wsMessage{
			Type: MsgTypeMarkerAWUpdated,
			Data: &tangleMarkerAWUpdated{
				ID:             markerAWUpdate.ID.Base58(),
				ApprovalWeight: markerAWUpdate.ApprovalWeight,
			},
		})

	})

	deps.Tangle.Storage.Events.MessageStored.Attach(storeClosure)
	deps.Tangle.Booker.Events.MessageBooked.Attach(bookedClosure)
	deps.Tangle.Booker.MarkersManager.Events.FutureMarkerUpdated.Attach(fmUpdateClosure)
	deps.Tangle.ApprovalWeightManager.Events.MarkerAWUpdated.Attach(markerConfirmedClosure)
	deps.Tangle.ApprovalWeightManager.Events.MessageFinalized.Attach(finalizedClosure)
}

func registerUTXOEvents() {
	storeClosure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			if msg.Payload().Type() == ledgerstate.TransactionType {
				tx := msg.Payload().(*ledgerstate.Transaction)
				// handle inputs (retrieve outputID)
				inputs := make([]*jsonmodels.Input, len(tx.Essence().Inputs()))
				for i, input := range tx.Essence().Inputs() {
					inputs[i] = jsonmodels.NewInput(input)
				}

				outputs := make([]string, len(tx.Essence().Outputs()))
				for i, output := range tx.Essence().Outputs() {
					outputs[i] = output.ID().Base58()
				}

				visualizerWorkerPool.TrySubmit(&wsMessage{
					Type: MsgTypeUTXOVertex,
					Data: &utxoVertex{
						MsgID:          messageID.Base58(),
						ID:             tx.ID().Base58(),
						Inputs:         inputs,
						Outputs:        outputs,
						ApprovalWeight: 0,
					},
				})
			}
		})
	})

	txConfirmedClosure := events.NewClosure(func(txID ledgerstate.TransactionID) {
		deps.Tangle.LedgerState.TransactionMetadata(txID).Consume(func(txMetadata *ledgerstate.TransactionMetadata) {
			visualizerWorkerPool.TrySubmit(&wsMessage{
				Type: MsgTypeUTXOConfirmed,
				Data: &utxoConfirmed{
					ID:             txID.Base58(),
					ApprovalWeight: 0,
					ConfirmedTime:  txMetadata.FinalizedTime().UnixNano(),
				},
			})
		})
	})

	deps.Tangle.Storage.Events.MessageStored.Attach(storeClosure)
	deps.Tangle.LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(txConfirmedClosure)
}

func registerBranchEvents() {
	createdClosure := events.NewClosure(func(branchDAGEvent *ledgerstate.BranchDAGEvent) {
		branchDAGEvent.Branch.Consume(func(branch ledgerstate.Branch) {
			conflicts := make(map[ledgerstate.ConflictID][]ledgerstate.BranchID)
			// get conflicts for Conflict branch
			if branch.Type() == ledgerstate.ConflictBranchType {
				for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
					conflicts[conflictID] = make([]ledgerstate.BranchID, 0)
					deps.Tangle.LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
						conflicts[conflictID] = append(conflicts[conflictID], conflictMember.BranchID())
					})
				}
			}

			visualizerWorkerPool.TrySubmit(&wsMessage{
				Type: MsgTypeBranchVertex,
				Data: &branchVertex{
					ID:             branch.ID().Base58(),
					Type:           branch.Type().String(),
					Parents:        branch.Parents().Strings(),
					ApprovalWeight: 0,
					Conflicts:      jsonmodels.NewGetBranchConflictsResponse(branch.ID(), conflicts),
				},
			})
		})
	})
	// BranchParentUpdate
	parentUpdateClosure := events.NewClosure(func(parentUpdate *ledgerstate.BranchParentUpdate) {
		visualizerWorkerPool.TrySubmit(&wsMessage{
			Type: MsgTypeBranchParentsUpdate,
			Data: &branchParentUpdate{
				ID:      parentUpdate.ID.Base58(),
				Parents: parentUpdate.NewParents.Strings(),
			},
		})
	})

	awUpdateClosure := events.NewClosure(func(branchAW *tangle.BranchAWUpdated) {
		visualizerWorkerPool.TrySubmit(&wsMessage{
			Type: MsgTypeBranchAWUpdate,
			Data: &branchAWUpdate{
				ID:             branchAW.ID.Base58(),
				Conflicts:      branchAW.Conflicts.Strings(),
				ApprovalWeight: branchAW.ApprovalWeight,
			},
		})
	})

	deps.Tangle.LedgerState.BranchDAG.Events.BranchCreated.Attach(createdClosure)
	deps.Tangle.LedgerState.BranchDAG.Events.BranchParentsUpdated.Attach(parentUpdateClosure)
	deps.Tangle.ApprovalWeightManager.Events.BranchAWUpdated.Attach(awUpdateClosure)
}
