package dashboard

import (
	"math"
	"sort"
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

type conflictCleanup struct {
	ConflictIDS []ledgerstate.ConflictID `json:"conflictIDs"`
}

type conflictCleanupJSON struct {
	ConflictIDS []string `json:"conflictIDs"`
}

func (c *conflictCleanup) ToJSON() *conflictCleanupJSON {
	var conflictIDS []string
	for _, c := range c.ConflictIDS {
		conflictIDS = append(conflictIDS, c.Base58())
	}
	return &conflictCleanupJSON{ConflictIDS: conflictIDS}
}

type branchCleanup struct {
	BranchIDs []ledgerstate.BranchID `json:"branchIDs"`
}

type branchCleanupJSON struct {
	BranchIDs []string `json:"branchIDs"`
}

func (b *branchCleanup) ToJSON() *branchCleanupJSON {
	var branchIDs []string
	for _, b := range b.BranchIDs {
		branchIDs = append(branchIDs, b.Base58())
	}
	return &branchCleanupJSON{BranchIDs: branchIDs}
}

func sendConflictUpdate(c *conflict) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsConflict, c.ToJSON())
}

func sendBranchUpdate(b *branch) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsBranch, b.ToJSON())
}

func sendConflictCleanup(c *conflictCleanup) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictCleanup, c.ToJSON())
}

func sendBranchCleanup(b *branchCleanup) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeBranchCleanup, b.ToJSON())
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

func cleanupOldConflicts(conflictsMap map[ledgerstate.ConflictID]*conflict) {
	var conflicts []*conflict
	for _, conflict := range conflictsMap {
		conflicts = append(conflicts, conflict)
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].UpdatedTime.Before(conflicts[i].UpdatedTime)
	})
	var conflictIDs []ledgerstate.ConflictID
	for _, c := range conflicts[:Parameters.Conflicts.ConflictCleanupCount] {
		conflictIDs = append(conflictIDs, c.ConflictID)
	}
	sendConflictCleanup(&conflictCleanup{ConflictIDS: conflictIDs})

	// free map
	for _, conflictID := range conflictIDs {
		delete(conflictsMap, conflictID)
	}
}

func cleanupOldBranches(branchesMap map[ledgerstate.BranchID]*branch, conflictsMap map[ledgerstate.ConflictID]*conflict) {
	var branches []*branch
	for _, branch := range branchesMap {
		branches = append(branches, branch)
	}
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].UpdatedTime.Before(branches[i].UpdatedTime)
	})
	var branchIDs []ledgerstate.BranchID
	for _, b := range branches[:Parameters.Conflicts.BranchCleanupCount] {
		branchIDs = append(branchIDs, b.BranchID)
	}
	sendBranchCleanup(&branchCleanup{BranchIDs: branchIDs})

	// follow up and cleanup conflicts
	conflictsToCleanup := make(map[ledgerstate.ConflictID]struct{})
	for _, branchID := range branchIDs {
		for _, conflictID := range branchesMap[branchID].ConflictIDs.Slice() {
			conflictsToCleanup[conflictID] = struct{}{}
		}
	}
	var conflictIDs []ledgerstate.ConflictID
	for conflictID := range conflictsToCleanup {
		conflictIDs = append(conflictIDs, conflictID)
	}
	sendConflictCleanup(&conflictCleanup{ConflictIDS: conflictIDs})

	// free branches map
	for _, branchID := range branchIDs {
		delete(branchesMap, branchID)
	}

	// free conflicts map
	for _, conflictID := range conflictIDs {
		delete(conflictsMap, conflictID)
	}
}
