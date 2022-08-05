package dashboard

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/clock"

	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

var (
	mu                               sync.RWMutex
	conflicts                        *boundedConflictMap
	conflictsLiveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	conflictsLiveFeedWorkerCount     = 1
	conflictsLiveFeedWorkerQueueSize = 50
)

type conflictSet struct {
	ConflictSetID utxo.OutputID `json:"conflictSetID"`
	ArrivalTime   time.Time     `json:"arrivalTime"`
	Resolved      bool          `json:"resolved"`
	TimeToResolve time.Duration `json:"timeToResolve"`
	UpdatedTime   time.Time     `json:"updatedTime"`
}

type conflictSetJSON struct {
	ConflictSetID string        `json:"conflictSetID"`
	ArrivalTime   int64         `json:"arrivalTime"`
	Resolved      bool          `json:"resolved"`
	TimeToResolve time.Duration `json:"timeToResolve"`
}

func (c *conflictSet) ToJSON() *conflictSetJSON {
	return &conflictSetJSON{
		ConflictSetID: c.ConflictSetID.Base58(),
		ArrivalTime:   c.ArrivalTime.Unix(),
		Resolved:      c.Resolved,
		TimeToResolve: c.TimeToResolve,
	}
}

type conflict struct {
	ConflictID        utxo.TransactionID              `json:"conflictID"`
	ConflictSetIDs    *set.AdvancedSet[utxo.OutputID] `json:"conflictSetIDs"`
	ConfirmationState confirmation.State              `json:"confirmationState"`
	IssuingTime       time.Time                       `json:"issuingTime"`
	IssuerNodeID      identity.ID                     `json:"issuerNodeID"`
	UpdatedTime       time.Time                       `json:"updatedTime"`
}

type conflictJSON struct {
	ConflictID        string             `json:"conflictID"`
	ConflictSetIDs    []string           `json:"conflictSetIDs"`
	ConfirmationState confirmation.State `json:"confirmationState"`
	IssuingTime       int64              `json:"issuingTime"`
	IssuerNodeID      string             `json:"issuerNodeID"`
}

func (c *conflict) ToJSON() *conflictJSON {
	return &conflictJSON{
		ConflictID: c.ConflictID.Base58(),
		ConflictSetIDs: func() (conflictSetIDsStr []string) {
			for it := c.ConflictSetIDs.Iterator(); it.HasNext(); {
				conflictSetID := it.Next()
				conflictSetIDsStr = append(conflictSetIDsStr, conflictSetID.Base58())
			}
			return
		}(),
		IssuingTime:       c.IssuingTime.Unix(),
		IssuerNodeID:      c.IssuerNodeID.String(),
		ConfirmationState: c.ConfirmationState,
	}
}

func sendConflictSetUpdate(c *conflictSet) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsConflictSet, c.ToJSON())
}

func sendConflictUpdate(b *conflict) {
	conflictsLiveFeedWorkerPool.TrySubmit(MsgTypeConflictsConflict, b.ToJSON())
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
			conflictSets: make(map[utxo.OutputID]*conflictSet),
			conflicts:    make(map[utxo.TransactionID]*conflict),
			conflictHeap: &timeHeap{},
		}

		onConflictCreatedClosure := event.NewClosure(onConflictCreated)
		onConflictAcceptedClosure := event.NewClosure(onConflictAccepted)
		onConflictRejectedClosure := event.NewClosure(onConflictRejected)
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(onConflictCreatedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictAccepted.Attach(onConflictAcceptedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictRejected.Attach(onConflictRejectedClosure)

		<-ctx.Done()

		log.Info("Stopping Dashboard[ConflictsLiveFeed] ...")
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Detach(onConflictCreatedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictAccepted.Detach(onConflictAcceptedClosure)
		deps.Tangle.Ledger.ConflictDAG.Events.ConflictRejected.Detach(onConflictRejectedClosure)
		log.Info("Stopping Dashboard[ConflictsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onConflictCreated(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
	conflictID := event.ID
	b := &conflict{
		ConflictID:  conflictID,
		UpdatedTime: clock.SyncedTime(),
	}

	deps.Tangle.Ledger.Storage.CachedTransaction(conflictID).Consume(func(transaction utxo.Transaction) {
		if tx, ok := transaction.(*devnetvm.Transaction); ok {
			b.IssuingTime = tx.Essence().Timestamp()
		}
	})

	// get the issuer of the oldest attachment of transaction that introduced the conflictSet
	b.IssuerNodeID = issuerOfOldestAttachment(conflictID)

	// now we update the shared data structure
	mu.Lock()
	defer mu.Unlock()

	deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflict(conflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		b.ConflictSetIDs = conflict.ConflictSetIDs()
	})

	for it := b.ConflictSetIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		_, exists := conflicts.conflictset(conflictID)
		// if this is the first conflictSet of this conflictSet set we add it to the map
		if !exists {
			c := &conflictSet{
				ConflictSetID: conflictID,
				ArrivalTime:   clock.SyncedTime(),
				UpdatedTime:   clock.SyncedTime(),
			}
			conflicts.addConflictSet(c)
		}

		// update all existing conflicts with a possible new conflictSet membership
		deps.Tangle.Ledger.ConflictDAG.Storage.CachedConflictMembers(conflictID).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {
			conflicts.addConflictMember(conflictMember.ConflictID(), conflictID)
		})
	}

	conflicts.addConflict(b)
}

func onConflictAccepted(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := conflicts.conflict(event.ID)
	if !exists {
		log.Warnf("conflict %s did not yet exist", event.ID)
		return
	}

	// if issuer is not yet set, set it now
	var id identity.ID
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.ConflictID)
	}

	b.ConfirmationState = confirmation.Accepted
	b.UpdatedTime = clock.SyncedTime()
	conflicts.addConflict(b)

	for it := b.ConflictSetIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		conflicts.resolveConflict(conflictID)
	}
}

func onConflictRejected(event *conflictdag.ConflictRejectedEvent[utxo.TransactionID]) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := conflicts.conflict(event.ID)
	if !exists {
		log.Warnf("conflict %s did not yet exist", event.ID)
		return
	}

	// if issuer is not yet set, set it now
	var id identity.ID
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.ConflictID)
	}

	b.ConfirmationState = confirmation.Rejected
	b.UpdatedTime = clock.SyncedTime()
	conflicts.addConflict(b)
}

// sendAllConflicts sends all conflicts and conflicts to the websocket.
func sendAllConflicts() {
	mu.RLock()
	defer mu.RUnlock()
	conflicts.sendAllConflicts()
}

func issuerOfOldestAttachment(conflictID utxo.TransactionID) (id identity.ID) {
	var oldestAttachmentTime time.Time
	deps.Tangle.Storage.Attachments(utxo.TransactionID(conflictID)).Consume(func(attachment *tangleold.Attachment) {
		deps.Tangle.Storage.Block(attachment.BlockID()).Consume(func(block *tangleold.Block) {
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
	conflictSets map[utxo.OutputID]*conflictSet
	conflicts    map[utxo.TransactionID]*conflict
	conflictHeap *timeHeap
}

func (b *boundedConflictMap) conflictset(conflictID utxo.OutputID) (conflict *conflictSet, exists bool) {
	conflict, exists = b.conflictSets[conflictID]
	return
}

func (b *boundedConflictMap) addConflictSet(c *conflictSet) {
	if len(b.conflictSets) >= Parameters.Conflicts.MaxCount {
		element := heap.Pop(b.conflictHeap).(*timeHeapElement)
		delete(b.conflictSets, element.conflictID)

		// cleanup conflicts. delete is a no-op if key is not found in map.
		for _, conflict := range b.conflicts {
			conflict.ConflictSetIDs.Delete(element.conflictID)
			if conflict.ConflictSetIDs.IsEmpty() {
				delete(b.conflicts, conflict.ConflictID)
			}
		}
	}
	b.conflictSets[c.ConflictSetID] = c
	heap.Push(b.conflictHeap, &timeHeapElement{
		conflictID:  c.ConflictSetID,
		arrivalTime: c.ArrivalTime,
	})
	sendConflictSetUpdate(c)
}

func (b *boundedConflictMap) resolveConflict(conflictID utxo.OutputID) {
	if c, exists := b.conflictSets[conflictID]; exists && !c.Resolved {
		c.Resolved = true
		c.TimeToResolve = clock.Since(c.ArrivalTime)
		c.UpdatedTime = clock.SyncedTime()
		b.conflictSets[conflictID] = c
		sendConflictSetUpdate(c)
	}
}

func (b *boundedConflictMap) sendAllConflicts() {
	for _, c := range b.conflictSets {
		sendConflictSetUpdate(c)
	}
	for _, b := range b.conflicts {
		sendConflictUpdate(b)
	}
}

func (b *boundedConflictMap) addConflict(br *conflict) {
	b.conflicts[br.ConflictID] = br
	sendConflictUpdate(br)
}

func (b *boundedConflictMap) addConflictMember(conflictID utxo.TransactionID, resourceID utxo.OutputID) {
	if _, exists := b.conflicts[conflictID]; exists {
		b.conflicts[conflictID].ConflictSetIDs.Add(resourceID)
		b.conflicts[conflictID].UpdatedTime = clock.SyncedTime()
		sendConflictUpdate(b.conflicts[conflictID])
	}
}

func (b *boundedConflictMap) conflict(conflictID utxo.TransactionID) (conflict *conflict, exists bool) {
	conflict, exists = b.conflicts[conflictID]
	return
}
