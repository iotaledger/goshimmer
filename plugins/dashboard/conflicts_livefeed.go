package dashboard

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

var (
	mu        sync.RWMutex
	conflicts *boundedConflictMap
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
	ConflictID        utxo.TransactionID                      `json:"conflictID"`
	ConflictSetIDs    *advancedset.AdvancedSet[utxo.OutputID] `json:"conflictSetIDs"`
	ConfirmationState confirmation.State                      `json:"confirmationState"`
	IssuingTime       time.Time                               `json:"issuingTime"`
	IssuerNodeID      identity.ID                             `json:"issuerNodeID"`
	UpdatedTime       time.Time                               `json:"updatedTime"`
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
	broadcastWsBlock(&wsblk{MsgTypeConflictsConflictSet, c.ToJSON()})
}

func sendConflictUpdate(b *conflict) {
	broadcastWsBlock(&wsblk{MsgTypeConflictsConflict, b.ToJSON()})
}

func runConflictLiveFeed(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dashboard[ConflictsLiveFeed]", func(ctx context.Context) {
		conflicts = &boundedConflictMap{
			conflictSets: make(map[utxo.OutputID]*conflictSet),
			conflicts:    make(map[utxo.TransactionID]*conflict),
			conflictHeap: &timeHeap{},
		}

		unhook := lo.Batch(
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictCreated.Hook(onConflictCreated, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(onConflictAccepted, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictRejected.Hook(onConflictRejected, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
		)

		<-ctx.Done()

		log.Info("Stopping Dashboard[ConflictsLiveFeed] ...")
		unhook()
		log.Info("Stopping Dashboard[ConflictsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onConflictCreated(c *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
	conflictID := c.ID()
	b := &conflict{
		ConflictID:  conflictID,
		UpdatedTime: time.Now(),
	}

	deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransaction(conflictID).Consume(func(transaction utxo.Transaction) {
		if tx, ok := transaction.(*devnetvm.Transaction); ok {
			b.IssuingTime = tx.Essence().Timestamp()
		}
	})

	// get the issuer of the oldest attachment of transaction that introduced the conflictSet
	b.IssuerNodeID = issuerOfOldestAttachment(conflictID)

	// now we update the shared data structure
	mu.Lock()
	defer mu.Unlock()

	b.ConflictSetIDs = utxo.NewOutputIDs(lo.Map(c.ConflictSets().Slice(), func(cs *conflictdag.ConflictSet[utxo.TransactionID, utxo.OutputID]) utxo.OutputID {
		return cs.ID()
	})...)

	for it := b.ConflictSetIDs.Iterator(); it.HasNext(); {
		conflictSetID := it.Next()
		_, exists := conflicts.conflictset(conflictSetID)
		// if this is the first conflictSet of this conflictSet set we add it to the map
		if !exists {
			c := &conflictSet{
				ConflictSetID: conflictSetID,
				ArrivalTime:   time.Now(),
				UpdatedTime:   time.Now(),
			}
			conflicts.addConflictSet(c)
		}

		// update all existing conflicts with a possible new conflictSet membership
		cs, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ConflictSet(conflictSetID)
		if !exists {
			continue
		}
		_ = cs.Conflicts().ForEach(func(element *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) (err error) {
			conflicts.addConflictMember(element.ID(), conflictSetID)
			return nil
		})
	}

	conflicts.addConflict(b)
}

func onConflictAccepted(c *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := conflicts.conflict(c.ID())
	if !exists {
		// log.Warnf("conflict %s did not yet exist", c.ID())
		return
	}

	// if issuer is not yet set, set it now
	var id identity.ID
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.ConflictID)
	}

	b.ConfirmationState = confirmation.Accepted
	b.UpdatedTime = time.Now()
	conflicts.addConflict(b)

	for it := b.ConflictSetIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		conflicts.resolveConflict(conflictID)
	}
}

func onConflictRejected(c *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
	mu.Lock()
	defer mu.Unlock()

	b, exists := conflicts.conflict(c.ID())
	if !exists {
		// log.Warnf("conflict %s did not yet exist", c.ID())
		return
	}

	// if issuer is not yet set, set it now
	var id identity.ID
	if b.IssuerNodeID == id {
		b.IssuerNodeID = issuerOfOldestAttachment(b.ConflictID)
	}

	b.ConfirmationState = confirmation.Rejected
	b.UpdatedTime = time.Now()
	conflicts.addConflict(b)
}

// sendAllConflicts sends all conflicts and conflicts to the websocket.
func sendAllConflicts() {
	mu.RLock()
	defer mu.RUnlock()
	conflicts.sendAllConflicts()
}

func issuerOfOldestAttachment(conflictID utxo.TransactionID) (id identity.ID) {
	block := deps.Protocol.Engine().Tangle.Booker().GetEarliestAttachment(conflictID)
	if block != nil {
		return block.IssuerID()
	}
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
		c.TimeToResolve = time.Since(c.ArrivalTime)
		c.UpdatedTime = time.Now()
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
		b.conflicts[conflictID].UpdatedTime = time.Now()
		sendConflictUpdate(b.conflicts[conflictID])
	}
}

func (b *boundedConflictMap) conflict(conflictID utxo.TransactionID) (conflict *conflict, exists bool) {
	conflict, exists = b.conflicts[conflictID]
	return
}
