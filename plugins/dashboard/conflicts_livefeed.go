package dashboard

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types/confirmation"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	mu                               sync.RWMutex
	conflicts                        *boundedConflictMap
	conflictsLiveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	conflictsLiveFeedWorkerCount     = 1
	conflictsLiveFeedWorkerQueueSize = 50
)

type conflictSet struct {
	ConflictID    utxo.OutputID `json:"conflictID"`
	ArrivalTime   time.Time     `json:"arrivalTime"`
	Resolved      bool          `json:"resolved"`
	TimeToResolve time.Duration `json:"timeToResolve"`
	UpdatedTime   time.Time     `json:"updatedTime"`
}

type conflictJSON struct {
	ConflictID    string        `json:"conflictID"`
	ArrivalTime   int64         `json:"arrivalTime"`
	Resolved      bool          `json:"resolved"`
	TimeToResolve time.Duration `json:"timeToResolve"`
}

func (c *conflictSet) ToJSON() *conflictJSON {
	return &conflictJSON{
		ConflictID:    c.ConflictID.Base58(),
		ArrivalTime:   c.ArrivalTime.Unix(),
		Resolved:      c.Resolved,
		TimeToResolve: c.TimeToResolve,
	}
}

type branch struct {
	BranchID          utxo.TransactionID              `json:"branchID"`
	ConflictIDs       *set.AdvancedSet[utxo.OutputID] `json:"conflictIDs"`
	ConfirmationState confirmation.State              `json:"confirmationState"`
	IssuingTime       time.Time                       `json:"issuingTime"`
	IssuerNodeID      identity.ID                     `json:"issuerNodeID"`
	UpdatedTime       time.Time                       `json:"updatedTime"`
}

type branchJSON struct {
	BranchID          string             `json:"branchID"`
	ConflictIDs       []string           `json:"conflictIDs"`
	ConfirmationState confirmation.State `json:"confirmationState"`
	IssuingTime       int64              `json:"issuingTime"`
	IssuerNodeID      string             `json:"issuerNodeID"`
}

func (b *branch) ToJSON() *branchJSON {
	return &branchJSON{
		BranchID: b.BranchID.Base58(),
		ConflictIDs: func() (conflictIDsStr []string) {
			for it := b.ConflictIDs.Iterator(); it.HasNext(); {
				conflictID := it.Next()
				conflictIDsStr = append(conflictIDsStr, conflictID.Base58())
			}
			return
		}(),
		IssuingTime:       b.IssuingTime.Unix(),
		IssuerNodeID:      b.IssuerNodeID.String(),
		ConfirmationState: b.ConfirmationState,
	}
}

func sendConflictSetUpdate(c *conflictSet) {
	conflictsLiveFeedWorkerPool.TrySubmit(BlkTypeConflictsConflict, c.ToJSON())
}

func sendBranchUpdate(b *branch) {
	conflictsLiveFeedWorkerPool.TrySubmit(BlkTypeConflictsBranch, b.ToJSON())
}

func configureConflictLiveFeed() {
	conflictsLiveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		broadcastWsBlock(&wsblk{task.Param(0).(byte), task.Param(1)})
		task.Return(nil)
	}, workerpool.WorkerCount(conflictsLiveFeedWorkerCount), workerpool.QueueSize(conflictsLiveFeedWorkerQueueSize))
}

func runConflictLiveFeed() {
	if err := daemon.BackgroundWorker("Dashboard[ConflictsLiveFeed]", func(ctx context.Context) {
		defer conflictsLiveFeedWorkerPool.Stop()

		conflicts = &boundedConflictMap{
			conflicts:    make(map[utxo.OutputID]*conflictSet),
			branches:     make(map[utxo.TransactionID]*branch),
			conflictHeap: &timeHeap{},
		}

		onBranchCreatedClosure := event.NewClosure(onBranchCreated)
		onBranchAcceptedClosure := event.NewClosure(onBranchAccepted)
		onBranchRejectedClosure := event.NewClosure(onBranchRejected)
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(onBranchCreatedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.BranchAccepted.Attach(onBranchAcceptedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(onBranchRejectedClosure)

		<-ctx.Done()

		log.Info("Stopping Dashboard[ConflictsLiveFeed] ...")
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Detach(onBranchCreatedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.BranchAccepted.Detach(onBranchAcceptedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.BranchRejected.Detach(onBranchRejectedClosure)
		log.Info("Stopping Dashboard[ConflictsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onBranchCreated(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
	branchID := event.ID
	b := &branch{
		BranchID:    branchID,
		UpdatedTime: clock.SyncedTime(),
	}

	deps.Tangle.Ledger.Storage.CachedTransaction(branchID).Consume(func(transaction utxo.Transaction) {
		if tx, ok := transaction.(*devnetvm.Transaction); ok {
			b.IssuingTime = tx.Essence().Timestamp()
		}
	})

	// get the issuer of the oldest attachment of transaction that introduced the conflictSet
	b.IssuerNodeID = issuerOfOldestAttachment(branchID)

	// now we update the shared data structure
	mu.Lock()
	defer mu.Unlock()

	deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflict(branchID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		b.ConflictIDs = conflict.ConflictSetIDs()
	})

	for it := b.ConflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		_, exists := conflicts.conflict(conflictID)
		// if this is the first conflictSet of this conflictSet set we add it to the map
		if !exists {
			c := &conflictSet{
				ConflictID:  conflictID,
				ArrivalTime: clock.SyncedTime(),
				UpdatedTime: clock.SyncedTime(),
			}
			conflicts.addConflictSet(c)
		}

		// update all existing branches with a possible new conflictSet membership
		deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflictMembers(conflictID).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {
			conflicts.addConflictMember(conflictMember.ConflictID(), conflictID)
		})
	}

	conflicts.addBranch(b)
}

func onBranchAccepted(event *conflictdag.BranchAcceptedEvent[utxo.TransactionID]) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := conflicts.branch(event.ID)
	if !exists {
		log.Warnf("branch %s did not yet exist", event.ID)
		return
	}

	// if issuer is not yet set, set it now
	var id identity.ID
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.BranchID)
	}

	b.ConfirmationState = confirmation.Accepted
	b.UpdatedTime = clock.SyncedTime()
	conflicts.addBranch(b)

	for it := b.ConflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		conflicts.resolveConflict(conflictID)
	}
}

func onBranchRejected(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := conflicts.branch(event.ID)
	if !exists {
		log.Warnf("branch %s did not yet exist", event.ID)
		return
	}

	// if issuer is not yet set, set it now
	var id identity.ID
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.BranchID)
	}

	b.ConfirmationState = confirmation.Rejected
	b.UpdatedTime = clock.SyncedTime()
	conflicts.addBranch(b)
}

// sendAllConflicts sends all conflicts and branches to the websocket.
func sendAllConflicts() {
	mu.RLock()
	defer mu.RUnlock()
	conflicts.sendAllConflicts()
}

func issuerOfOldestAttachment(branchID utxo.TransactionID) (id identity.ID) {
	var oldestAttachmentTime time.Time
	deps.Tangle.Storage.Attachments(utxo.TransactionID(branchID)).Consume(func(attachment *tangle.Attachment) {
		deps.Tangle.Storage.Block(attachment.BlockID()).Consume(func(block *tangle.Block) {
			if oldestAttachmentTime.IsZero() || block.IssuingTime().Before(oldestAttachmentTime) {
				oldestAttachmentTime = block.IssuingTime()
				id = identity.New(block.IssuerPublicKey()).ID()
			}
		})
	})
	return
}

type timeHeapElement struct {
	conflictID  utxo.OutputID
	arrivalTime time.Time
}

type timeHeap []*timeHeapElement

func (h timeHeap) Len() int {
	return len(h)
}

func (h timeHeap) Less(i, j int) bool {
	return h[i].arrivalTime.Before(h[j].arrivalTime)
}

func (h timeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *timeHeap) Push(x interface{}) {
	*h = append(*h, x.(*timeHeapElement))
}

func (h *timeHeap) Pop() interface{} {
	n := len(*h)
	data := (*h)[n-1]
	(*h)[n-1] = nil
	*h = (*h)[:n-1]
	return data
}

var _ heap.Interface = &timeHeap{}

type boundedConflictMap struct {
	conflicts    map[utxo.OutputID]*conflictSet
	branches     map[utxo.TransactionID]*branch
	conflictHeap *timeHeap
}

func (b *boundedConflictMap) conflict(conflictID utxo.OutputID) (conflict *conflictSet, exists bool) {
	conflict, exists = b.conflicts[conflictID]
	return
}

func (b *boundedConflictMap) addConflictSet(c *conflictSet) {
	if len(b.conflicts) >= Parameters.Conflicts.MaxCount {
		element := heap.Pop(b.conflictHeap).(*timeHeapElement)
		delete(b.conflicts, element.conflictID)

		// cleanup branches. delete is a no-op if key is not found in map.
		for _, branch := range b.branches {
			branch.ConflictIDs.Delete(element.conflictID)
			if branch.ConflictIDs.IsEmpty() {
				delete(b.branches, branch.BranchID)
			}
		}
	}
	b.conflicts[c.ConflictID] = c
	heap.Push(b.conflictHeap, &timeHeapElement{
		conflictID:  c.ConflictID,
		arrivalTime: c.ArrivalTime,
	})
	sendConflictSetUpdate(c)
}

func (b *boundedConflictMap) resolveConflict(conflictID utxo.OutputID) {
	if c, exists := b.conflicts[conflictID]; exists && !c.Resolved {
		c.Resolved = true
		c.TimeToResolve = clock.Since(c.ArrivalTime)
		c.UpdatedTime = clock.SyncedTime()
		b.conflicts[conflictID] = c
		sendConflictSetUpdate(c)
	}
}

func (b *boundedConflictMap) sendAllConflicts() {
	for _, c := range b.conflicts {
		sendConflictSetUpdate(c)
	}
	for _, b := range b.branches {
		sendBranchUpdate(b)
	}
}

func (b *boundedConflictMap) addBranch(br *branch) {
	b.branches[br.BranchID] = br
	sendBranchUpdate(br)
}

func (b *boundedConflictMap) addConflictMember(branchID utxo.TransactionID, conflictID utxo.OutputID) {
	if _, exists := b.branches[branchID]; exists {
		b.branches[branchID].ConflictIDs.Add(conflictID)
		b.branches[branchID].UpdatedTime = clock.SyncedTime()
		sendBranchUpdate(b.branches[branchID])
	}
}

func (b *boundedConflictMap) branch(branchID utxo.TransactionID) (branch *branch, exists bool) {
	branch, exists = b.branches[branchID]
	return
}
