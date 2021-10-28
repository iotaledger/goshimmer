package dashboard

import (
	"container/heap"
	"math"
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

const precision float64 = 1000

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
	UpdatedTime   time.Time              `json:"updatedTime"`
}

type conflictJSON struct {
	ConflictID    string        `json:"conflictID"`
	ArrivalTime   int64         `json:"arrivalTime"`
	Resolved      bool          `json:"resolved"`
	TimeToResolve time.Duration `json:"timeToResolve"`
}

func (c *conflict) ToJSON() *conflictJSON {
	return &conflictJSON{
		ConflictID:    c.ConflictID.Base58(),
		ArrivalTime:   c.ArrivalTime.Unix(),
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
	UpdatedTime  time.Time               `json:"updatedTime"`
}

type branchJSON struct {
	BranchID     string              `json:"branchID"`
	ConflictIDs  []string            `json:"conflictIDs"`
	AW           float64             `json:"aw"`
	GoF          gof.GradeOfFinality `json:"gof"`
	IssuingTime  int64               `json:"issuingTime"`
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
		IssuingTime:  b.IssuingTime.Unix(),
		IssuerNodeID: b.IssuerNodeID.String(),
		AW:           b.AW,
		GoF:          b.GoF,
	}
}

func sendConflictUpdate(c *conflict) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsConflict, c.ToJSON())
}

func sendBranchUpdate(b *branch) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsBranch, b.ToJSON())
}

func configureConflictLiveFeed() {
	if Parameters.Conflicts.ConflictCleanupCount >= Parameters.Conflicts.MaxConflictsCount {
		Plugin.LogFatal("conflicts cleanup count must be less than max conflicts count")
	}
	if Parameters.Conflicts.BranchCleanupCount >= Parameters.Conflicts.MaxBranchesCount {
		Plugin.LogFatal("branch cleanup count must be less than max branches count")
	}
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

		onBranchCreatedClosure := events.NewClosure(onBranchCreated)
		onBranchWeightChangedClosure := events.NewClosure(onBranchWeightChanged)
		deps.Tangle.LedgerState.BranchDAG.Events.BranchCreated.Attach(onBranchCreatedClosure)
		deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.AttachAfter(onBranchWeightChangedClosure)

		<-shutdownSignal

		log.Info("Stopping Dashboard[ConflictsLiveFeed] ...")
		deps.Tangle.LedgerState.BranchDAG.Events.BranchCreated.Detach(onBranchCreatedClosure)
		deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.Detach(onBranchWeightChangedClosure)
		log.Info("Stopping Dashboard[ConflictsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onBranchCreated(branchID ledgerstate.BranchID) {
	b := &branch{
		BranchID:    branchID,
		UpdatedTime: clock.SyncedTime(),
	}

	deps.Tangle.LedgerState.Transaction(ledgerstate.TransactionID(branchID)).Consume(func(transaction *ledgerstate.Transaction) {
		b.IssuingTime = transaction.Essence().Timestamp()
	})

	// get the issuer of the oldest attachment of transaction that introduced the conflict
	b.IssuerNodeID = issuerOfOldestAttachment(branchID)

	// now we update the shared data structure
	mu.Lock()
	defer mu.Unlock()

	deps.Tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		b.ConflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()

		for conflictID := range b.ConflictIDs {
			_, exists := conflicts[conflictID]
			// if this is the first conflict of this conflict set we add it to the map
			if !exists {
				c := &conflict{
					ConflictID:  conflictID,
					ArrivalTime: clock.SyncedTime(),
					UpdatedTime: clock.SyncedTime(),
				}
				conflicts[conflictID] = c
				sendConflictUpdate(c)
				if len(conflicts) >= Parameters.Conflicts.MaxConflictsCount {
					cleanupOldConflicts(conflicts)
				}
			}

			// update all existing branches with a possible new conflict membership
			deps.Tangle.LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
				if cm, exists := branches[conflictMember.BranchID()]; exists {
					cm.ConflictIDs.Add(conflictID)
					cm.UpdatedTime = clock.SyncedTime()
					sendBranchUpdate(cm)
				}
			})
		}
	})

	branches[b.BranchID] = b
	sendBranchUpdate(b)
	if len(branches) >= Parameters.Conflicts.MaxBranchesCount {
		cleanupOldBranches(branches, conflicts)
	}
}

func onBranchWeightChanged(e *tangle.BranchWeightChangedEvent) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := branches[e.BranchID]
	if !exists {
		log.Warnf("branch %s did not yet exist", e.BranchID)
		return
	}

	var id identity.ID
	// if issuer is not yet set, set it now
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.BranchID)
	}

	b.AW = math.Round(e.Weight*precision) / precision
	b.GoF, _ = deps.Tangle.LedgerState.UTXODAG.BranchGradeOfFinality(b.BranchID)
	b.UpdatedTime = clock.SyncedTime()
	sendBranchUpdate(b)

	if messagelayer.FinalityGadget().IsBranchConfirmed(b.BranchID) {
		for conflictID := range b.ConflictIDs {
			c := conflicts[conflictID]
			if !c.Resolved {
				c.Resolved = true
				c.TimeToResolve = clock.Since(c.ArrivalTime)
				c.UpdatedTime = clock.SyncedTime()
				sendConflictUpdate(c)
			}
		}
	}
}

// sendAllConflicts sends all conflicts and branches to the websocket.
func sendAllConflicts() {
	mu.RLock()
	defer mu.RUnlock()

	for _, c := range conflicts {
		sendConflictUpdate(c)
	}
	for _, b := range branches {
		sendBranchUpdate(b)
	}
}

func issuerOfOldestAttachment(branchID ledgerstate.BranchID) (id identity.ID) {
	var oldestAttachmentTime time.Time
	deps.Tangle.Storage.Attachments(ledgerstate.TransactionID(branchID)).Consume(func(attachment *tangle.Attachment) {
		deps.Tangle.Storage.Message(attachment.MessageID()).Consume(func(message *tangle.Message) {
			if oldestAttachmentTime.IsZero() || message.IssuingTime().Before(oldestAttachmentTime) {
				oldestAttachmentTime = message.IssuingTime()
				id = identity.New(message.IssuerPublicKey()).ID()
			}
		})
	})
	return
}

type timeHeap []interface{}

func (h timeHeap) Len() int {
	return len(h)
}

func (h timeHeap) Less(i, j int) bool {
	switch h[i].(type) {
	case *conflict:
		return h[i].(*conflict).UpdatedTime.Before(h[j].(*conflict).UpdatedTime)
	}
	return h[i].(*branch).UpdatedTime.Before(h[j].(*branch).UpdatedTime)
}

func (h timeHeap) Swap(i, j int) {
	h[i], h[j] = h[i], h[j]
}

func (h *timeHeap) Push(x interface{}) {
	*h = append(*h, x)
}

func (h *timeHeap) Pop() interface{} {
	n := len(*h)
	data := (*h)[n-1]
	(*h)[n-1] = nil
	*h = (*h)[:n-1]
	return data
}

var _ heap.Interface = &timeHeap{}

func cleanupOldConflicts(conflictsMap map[ledgerstate.ConflictID]*conflict) {
	var conflictHeap = &timeHeap{}
	heap.Init(conflictHeap)
	for _, conflict := range conflictsMap {
		heap.Push(conflictHeap, conflict)
	}

	for i := 0; i < Parameters.Conflicts.ConflictCleanupCount; i++ {
		conflict := heap.Pop(conflictHeap).(*conflict)
		delete(conflictsMap, conflict.ConflictID)
	}
}

func cleanupOldBranches(branchesMap map[ledgerstate.BranchID]*branch, conflictsMap map[ledgerstate.ConflictID]*conflict) {
	var branchHeap = &timeHeap{}
	heap.Init(branchHeap)
	for _, branch := range branchesMap {
		heap.Push(branchHeap, branch)
	}
	for i := 0; i < Parameters.Conflicts.BranchCleanupCount; i++ {
		branch := heap.Pop(branchHeap).(*branch)
		delete(branchesMap, branch.BranchID)
		for _, conflictID := range branchesMap[branch.BranchID].ConflictIDs.Slice() {
			delete(conflictsMap, conflictID)
		}
	}
}
