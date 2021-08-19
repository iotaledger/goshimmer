package dashboard

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/iotaledger/goshimmer/packages/clock"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

var (
	mu                          sync.RWMutex
	conflicts                   map[ledgerstate.ConflictID]*conflict
	branches                    map[ledgerstate.BranchID]*branch
	conflictsLiveFeedWorkerPool *workerpool.NonBlockingQueuedWorkerPool
)

type conflict struct {
	ConflictID    ledgerstate.ConflictID `json:"conflictID"`
	ArrivalTime   time.Time              `json:"arrivalTime"`
	Resolved      bool                   `json:"resolved"`
	TimeToResolve time.Duration          `json:"timeToResolve"`
}

func (c *conflict) MarshalJSON() ([]byte, error) {
	type Alias conflict
	return json.Marshal(&struct {
		ConflictID  string `json:"conflictID"`
		ArrivalTime int64  `json:"arrivalTime"`
		*Alias
	}{
		ConflictID:  c.ConflictID.Base58(),
		ArrivalTime: c.ArrivalTime.Unix(),
		Alias:       (*Alias)(c),
	})
}

type branch struct {
	BranchID     ledgerstate.BranchID    `json:"branchID"`
	ConflictIDs  ledgerstate.ConflictIDs `json:"conflictIDs"`
	AW           float64                 `json:"aw"`
	GoF          gof.GradeOfFinality     `json:"gof"`
	IssuingTime  time.Time               `json:"issuingTime"`
	IssuerNodeID identity.ID             `json:"issuerNodeID"`
}

func (b *branch) MarshalJSON() ([]byte, error) {
	type Alias branch
	return json.Marshal(&struct {
		BranchID     string   `json:"branchID"`
		ConflictIDs  []string `json:"conflictIDs"`
		IssuingTime  int64    `json:"issuingTime"`
		IssuerNodeID string   `json:"issuerNodeID"`
		*Alias
	}{
		BranchID: b.BranchID.Base58(),
		ConflictIDs: func() (conflictIDsStr []string) {
			for conflictID := range b.ConflictIDs {
				conflictIDsStr = append(conflictIDsStr, conflictID.Base58())
			}
			return
		}(),
		IssuingTime:  b.IssuingTime.Unix(),
		IssuerNodeID: b.IssuerNodeID.String(),
		Alias:        (*Alias)(b),
	})
}

func sendConflictUpdate(c *conflict) {
	fmt.Println(c)
}

func sendBranchUpdate(b *branch) {
	fmt.Println(b)
}

func configureConflictLiveFeed() {
	conflicts = make(map[ledgerstate.ConflictID]*conflict)
	branches = make(map[ledgerstate.BranchID]*branch)

	messagelayer.Tangle().LedgerState.BranchDAG.Events.BranchCreated.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
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
	}))

	messagelayer.Tangle().ApprovalWeightManager.Events.BranchWeightChanged.AttachAfter(events.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
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
	}))

	//conflictsLiveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
	//	newMessage := task.Param(0).(*chat.ChatEvent)
	//
	//	broadcastWsMessage(&wsmsg{MsgTypeChat, &chatMsg{
	//		From:      newMessage.From,
	//		To:        newMessage.To,
	//		Message:   newMessage.Message,
	//		MessageID: newMessage.MessageID,
	//		Timestamp: newMessage.Timestamp.Format("2 Jan 2006 15:04:05"),
	//	}})
	//
	//	task.Return(nil)
	//}, workerpool.WorkerCount(chatLiveFeedWorkerCount), workerpool.QueueSize(chatLiveFeedWorkerQueueSize))
}

func runConflictLiveFeed() {
	//if err := daemon.BackgroundWorker("Dashboard[ChatUpdater]", func(shutdownSignal <-chan struct{}) {
	//	notifyNewMessages := events.NewClosure(func(chatEvent *chat.ChatEvent) {
	//		chatLiveFeedWorkerPool.TrySubmit(chatEvent)
	//	})
	//	chat.Events.MessageReceived.Attach(notifyNewMessages)
	//
	//	defer chatLiveFeedWorkerPool.Stop()
	//
	//	<-shutdownSignal
	//	log.Info("Stopping Dashboard[ChatUpdater] ...")
	//	chat.Events.MessageReceived.Detach(notifyNewMessages)
	//	log.Info("Stopping Dashboard[ChatUpdater] ... done")
	//}, shutdown.PriorityDashboard); err != nil {
	//	log.Panicf("Failed to start as daemon: %s", err)
	//}
}
