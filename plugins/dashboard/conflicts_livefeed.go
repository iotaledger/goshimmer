package dashboard

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	mu                               sync.RWMutex
	conflicts                        map[ledgerstate.ConflictID]*conflict
	branches                         map[ledgerstate.BranchID]*branch
	conflictsLiveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	conflictsLiveFeedWorkerCount     = 1
	conflictsLiveFeedWorkerQueueSize = 50
)

type conflict struct {
	ConflictID    ledgerstate.ConflictID `json:"conflictID"`
	ArrivalTime   time.Time              `json:"arrivalTime"`
	Resolved      bool                   `json:"resolved"`
	TimeToResolve time.Duration          `json:"timeToResolve"`
}

type conflictJSON struct {
	ConflictID    string        `json:"conflictID"`
	ArrivalTime   string        `json:"arrivalTime"`
	Resolved      bool          `json:"resolved"`
	TimeToResolve time.Duration `json:"timeToResolve"`
}

func (c *conflict) ToJSON() *conflictJSON {
	return &conflictJSON{
		ConflictID:    c.ConflictID.Base58(),
		ArrivalTime:   c.ArrivalTime.Format("2 Jan 2006 15:04:05"),
		Resolved:      c.Resolved,
		TimeToResolve: c.TimeToResolve,
	}
}

type branch struct {
	BranchID     ledgerstate.BranchID    `json:"branchID"`
	ConflictIDs  ledgerstate.ConflictIDs `json:"conflictIDs"`
	AW           float64                 `json:"aw"`
	GoF          gof.GradeOfFinality     `json:"gof"`
	IssuingTime  time.Time               `json:"issuingTime"`
	IssuerNodeID identity.ID             `json:"issuerNodeID"`
}

type branchJSON struct {
	BranchID     string              `json:"branchID"`
	ConflictIDs  []string            `json:"conflictIDs"`
	AW           float64             `json:"aw"`
	GoF          gof.GradeOfFinality `json:"gof"`
	IssuingTime  string              `json:"issuingTime"`
	IssuerNodeID string              `json:"issuerNodeID"`
}

func (b *branch) ToJSON() *branchJSON {
	return &branchJSON{
		BranchID: b.BranchID.Base58(),
		ConflictIDs: func() (conflictIDsStr []string) {
			for conflictID := range b.ConflictIDs {
				conflictIDsStr = append(conflictIDsStr, conflictID.Base58())
			}
			return
		}(),
		IssuingTime:  b.IssuingTime.Format("2 Jan 2006 15:04:05"),
		IssuerNodeID: b.IssuerNodeID.String(),
		AW:           b.AW,
		GoF:          b.GoF,
	}
}

func sendConflictUpdate(c *conflict) {
	fmt.Println(c)
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsConflict, c.ToJSON())
}

func sendBranchUpdate(b *branch) {
	fmt.Println(b)
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsBranch, b.ToJSON())
}

func configureConflictLiveFeed() {
	conflictsLiveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		broadcastWsMessage(&wsmsg{task.Param(0).(byte), task.Param(1)})
		task.Return(nil)
	}, workerpool.WorkerCount(conflictsLiveFeedWorkerCount), workerpool.QueueSize(conflictsLiveFeedWorkerQueueSize))
}

func runConflictLiveFeed() {
	if err := daemon.BackgroundWorker("Dashboard[ConflictsLiveFeed]", func(shutdownSignal <-chan struct{}) {
		defer conflictsLiveFeedWorkerPool.Stop()

		conflicts = make(map[ledgerstate.ConflictID]*conflict)
		branches = make(map[ledgerstate.BranchID]*branch)

		onBranchCreated := events.NewClosure(func(branchID ledgerstate.BranchID) {
			b := &branch{
				BranchID: branchID,
			}

			messagelayer.Tangle().LedgerState.Transaction(ledgerstate.TransactionID(branchID)).Consume(func(transaction *ledgerstate.Transaction) {
				b.IssuingTime = transaction.Essence().Timestamp()
			})

			oldestAttachmentTime := time.Unix(0, 0)
			// get the oldest attachment of transaction that introduced the conflict
			messagelayer.Tangle().Storage.Attachments(ledgerstate.TransactionID(branchID)).Consume(func(attachment *tangle.Attachment) {
				messagelayer.Tangle().Storage.Message(attachment.MessageID()).Consume(func(message *tangle.Message) {
					if oldestAttachmentTime.Unix() == 0 || message.IssuingTime().Before(oldestAttachmentTime) {
						oldestAttachmentTime = message.IssuingTime()
						b.IssuerNodeID = identity.New(message.IssuerPublicKey()).ID()
					}
				})
			})

			// now we update the shared data structure
			mu.Lock()
			defer mu.Unlock()

			messagelayer.Tangle().LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
				b.ConflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()

				for conflictID := range b.ConflictIDs {
					c, exists := conflicts[conflictID]
					// if this is the first conflict of this conflict set we add it to the map
					if !exists {
						c = &conflict{
							ConflictID:  conflictID,
							ArrivalTime: clock.SyncedTime(),
						}
						conflicts[conflictID] = c
						sendConflictUpdate(c)
					}

					// update all existing branches with a possible new conflict membership
					messagelayer.Tangle().LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
						if cm, exists := branches[conflictMember.BranchID()]; exists {
							cm.ConflictIDs.Add(conflictID)
							sendBranchUpdate(cm)
						}
					})
				}
			})

			branches[b.BranchID] = b
			sendBranchUpdate(b)
		})

		messagelayer.Tangle().LedgerState.BranchDAG.Events.BranchCreated.Attach(onBranchCreated)

		onBranchWeightChanged := events.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
			mu.Lock()
			defer mu.Unlock()

			b, exists := branches[e.BranchID]
			if !exists {
				plugin.LogWarnf("branch %s did not yet exist", e.BranchID)
				return
			}

			b.AW = e.Weight
			messagelayer.Tangle().LedgerState.BranchDAG.Branch(b.BranchID).Consume(func(branch ledgerstate.Branch) {
				b.GoF = branch.GradeOfFinality()
			})
			sendBranchUpdate(b)

			if messagelayer.FinalityGadget().IsBranchConfirmed(b.BranchID) {
				for conflictID := range b.ConflictIDs {
					c := conflicts[conflictID]
					c.Resolved = true
					c.TimeToResolve = clock.Since(c.ArrivalTime)
					sendConflictUpdate(c)
				}
			}
		})

		messagelayer.Tangle().ApprovalWeightManager.Events.BranchWeightChanged.AttachAfter(onBranchWeightChanged)

		<-shutdownSignal

		log.Info("Stopping Dashboard[ConflictsLiveFeed] ...")
		messagelayer.Tangle().LedgerState.BranchDAG.Events.BranchCreated.Detach(onBranchCreated)
		messagelayer.Tangle().ApprovalWeightManager.Events.BranchWeightChanged.Detach(onBranchWeightChanged)
		log.Info("Stopping Dashboard[ConflictsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
