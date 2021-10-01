package dagsvisualizer

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			visualizerWorkerPool.TrySubmit(&wsMessage{
				Type: MsgTypeTangleVertex,
				Data: &tangleVertex{
					ID:              messageID.Base58(),
					StrongParentIDs: msg.StrongParents().ToStrings(),
					WeakParentIDs:   msg.WeakParents().ToStrings(),
				},
			})
		})
	})

	bookedClosure := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
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
		messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(msgMetadata *tangle.MessageMetadata) {
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

	messagelayer.Tangle().Storage.Events.MessageStored.Attach(storeClosure)
	messagelayer.Tangle().Booker.Events.MessageBooked.Attach(bookedClosure)
	messagelayer.Tangle().Booker.MarkersManager.Events.FutureMarkerUpdated.Attach(fmUpdateClosure)
	messagelayer.Tangle().ApprovalWeightManager.Events.MarkerAWUpdated.Attach(markerConfirmedClosure)
	messagelayer.Tangle().ApprovalWeightManager.Events.MessageFinalized.Attach(finalizedClosure)
}

func registerUTXOEvents() {
	storeClosure := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			if msg.Payload().Type() == ledgerstate.TransactionType {
				tx := msg.Payload().(*ledgerstate.Transaction)
				visualizerWorkerPool.TrySubmit(&wsMessage{
					Type: MsgTypeUTXOVertex,
					Data: &utxoVertex{
						MsgID:          messageID.Base58(),
						ID:             tx.ID().Base58(),
						Inputs:         tx.Essence().Inputs().Strings(),
						Outputs:        tx.Essence().Outputs().Strings(),
						ApprovalWeight: 0,
					},
				})
			}
		})
	})

	txConfirmedClosure := events.NewClosure(func(txID ledgerstate.TransactionID) {
		messagelayer.Tangle().LedgerState.TransactionMetadata(txID).Consume(func(txMetadata *ledgerstate.TransactionMetadata) {
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

	messagelayer.Tangle().Storage.Events.MessageStored.Attach(storeClosure)
	messagelayer.Tangle().LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(txConfirmedClosure)
}

func registerBranchEvents() {
	createdClosure := events.NewClosure(func(branchDAGEvent *ledgerstate.BranchDAGEvent) {
		branchDAGEvent.Branch.Consume(func(branch ledgerstate.Branch) {
			visualizerWorkerPool.TrySubmit(&wsMessage{
				Type: MsgTypeBranchVertex,
				Data: &branchVertex{
					ID:             branch.ID().Base58(),
					Parents:        branch.Parents().Strings(),
					ApprovalWeight: 0,
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

	messagelayer.Tangle().LedgerState.BranchDAG.Events.BranchCreated.Attach(createdClosure)
	messagelayer.Tangle().LedgerState.BranchDAG.Events.BranchParentsUpdated.Attach(parentUpdateClosure)
}
