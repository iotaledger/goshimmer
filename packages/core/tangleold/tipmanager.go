package tangleold

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/node/clock"
)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

const tipLifeGracePeriod = maxParentsTimeDifference - 1*time.Minute

// TipManager manages a map of tips and emits events for their removal and addition.
type TipManager struct {
	tangle              *Tangle
	tips                *randommap.RandomMap[BlockID, BlockID]
	tipsCleaner         *TipsCleaner
	tipsConflictTracker *TipsConflictTracker
	Events              *TipManagerEvents
}

// NewTipManager creates a new tip-selector.
func NewTipManager(tangle *Tangle, tips ...BlockID) *TipManager {
	tipManager := &TipManager{
		tangle:              tangle,
		tips:                randommap.New[BlockID, BlockID](),
		tipsConflictTracker: NewTipsConflictTracker(tangle),
		Events:              newTipManagerEvents(),
	}
	tipManager.tipsCleaner = &TipsCleaner{
		heap:       make([]*QueueElement, 0),
		tipManager: tipManager,
	}
	if tips != nil {
		tipManager.set(tips...)
	}

	return tipManager
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TipManager) Setup() {
	t.tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *BlockScheduledEvent) {
		t.tangle.Storage.Block(event.BlockID).Consume(t.AddTip)
	}))

	t.tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(event *BlockAcceptedEvent) {
		t.removeStrongParents(event.Block)
	}))

	t.tangle.OrphanageManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
		t.deleteTip(event.Block.ID())
	}))

	t.tangle.TimeManager.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *TimeUpdate) {
		t.tipsCleaner.RemoveBefore(event.UpdateTime.Add(-t.tangle.Options.TimeSinceConfirmationThreshold))
	}))

	t.tangle.OrphanageManager.Events.AllChildrenOrphaned.Hook(event.NewClosure(func(block *Block) {
		if clock.Since(block.IssuingTime()) > tipLifeGracePeriod {
			return
		}

		t.addTip(block)
	}))

	t.tipsConflictTracker.Setup()
}

// set adds the given blockIDs as tips.
func (t *TipManager) set(tips ...BlockID) {
	for _, blockID := range tips {
		t.tips.Set(blockID, blockID)
	}
}

// AddTip adds the block to the tip pool if its issuing time is within the tipLifeGracePeriod.
// Parents of a block that are currently tip lose the tip status and are removed.
func (t *TipManager) AddTip(block *Block) {
	blockID := block.ID()

	if clock.Since(block.IssuingTime()) > tipLifeGracePeriod {
		return
	}

	// Check if any children that are confirmed or scheduled and return if true, to guarantee that the parents are not added to the tipset after its children.
	if t.checkChildren(blockID) {
		return
	}

	if !t.addTip(block) {
		return
	}

	// skip removing tips if TangleWidth is enabled
	if t.TipCount() <= t.tangle.Options.TangleWidth {
		return
	}

	// a tip loses its tip status if it is referenced by another block
	t.removeStrongParents(block)
}

func (t *TipManager) addTip(block *Block) (added bool) {
	blockID := block.ID()

	var invalid bool
	t.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		invalid = blockMetadata.IsSubjectivelyInvalid()
	})
	if invalid {
		// fmt.Println("TipManager: skipping adding tip because it is subjectively invalid", blockID)
		return false
	}

	if t.tips.Set(blockID, blockID) {
		t.tipsConflictTracker.AddTip(blockID)
		t.Events.TipAdded.Trigger(&TipEvent{
			BlockID: blockID,
		})

		t.tipsCleaner.Add(block.IssuingTime(), blockID)
		return true
	}

	return false
}

func (t *TipManager) deleteTip(blkID BlockID) (deleted bool) {
	if _, deleted = t.tips.Delete(blkID); deleted {
		t.tipsConflictTracker.RemoveTip(blkID)
		t.Events.TipRemoved.Trigger(&TipEvent{
			BlockID: blkID,
		})
	}
	return
}

// checkChildren returns true if the block has any confirmed or scheduled child.
func (t *TipManager) checkChildren(blockID BlockID) bool {
	childScheduledConfirmed := false
	t.tangle.Storage.Children(blockID).Consume(func(child *Child) {
		if childScheduledConfirmed {
			return
		}

		childScheduledConfirmed = t.tangle.ConfirmationOracle.IsBlockConfirmed(child.ChildBlockID())
		if !childScheduledConfirmed {
			t.tangle.Storage.BlockMetadata(child.ChildBlockID()).Consume(func(blockMetadata *BlockMetadata) {
				childScheduledConfirmed = blockMetadata.Scheduled()
			})
		}
	})
	return childScheduledConfirmed
}

func (t *TipManager) removeStrongParents(block *Block) {
	block.ForEachParentByType(StrongParentType, func(parentBlockID BlockID) bool {
		// We do not want to remove the tip if it is the last one representing a pending conflict.
		// if t.isLastTipForConflict(parentBlockID) {
		// 	return true
		// }

		t.deleteTip(parentBlockID)

		return true
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(p payload.Payload, countParents int) (parents BlockIDs) {
	if countParents > MaxParentsCount {
		countParents = MaxParentsCount
	}
	if countParents < MinParentsCount {
		countParents = MinParentsCount
	}

	return t.selectTips(p, countParents)
}

// isPastConeTimestampCorrect performs the TSC check for the given tip.
// Conceptually, this involves the following steps:
//   1. Collect all confirmed blocks in the tip's past cone at the boundary of confirmed/unconfirmed.
//   2. Order by timestamp (ascending), if the oldest confirmed block > TSC threshold then return false.
//
// This function is optimized through the use of markers and the following assumption:
//   If there's any unconfirmed block >TSC threshold, then the oldest confirmed block will be >TSC threshold, too.
func (t *TipManager) isPastConeTimestampCorrect(blockID BlockID) (timestampValid bool) {
	minSupportedTimestamp := t.tangle.TimeManager.ATT().Add(-t.tangle.Options.TimeSinceConfirmationThreshold)
	timestampValid = true

	// skip TSC check if no block has been confirmed to allow attaching to genesis
	if t.tangle.TimeManager.LastAcceptedBlock().BlockID == EmptyBlockID {
		// if the genesis block is the last confirmed block, then there is no point in performing tangle walk
		// return true so that the network can start issuing blocks when the tangle starts
		return true
	}

	t.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		timestampValid = block.IssuingTime().After(minSupportedTimestamp)
	})

	if !timestampValid {
		return false
	}
	if t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
		// return true if block is confirmed and has valid timestamp
		return true
	}

	markerWalker := walker.New[markers.Marker](false)
	blockWalker := walker.New[BlockID](false)

	t.processBlock(blockID, blockWalker, markerWalker)
	previousBlockID := blockID
	for markerWalker.HasNext() && timestampValid {
		marker := markerWalker.Next()
		previousBlockID, timestampValid = t.checkMarker(marker, previousBlockID, blockWalker, markerWalker, minSupportedTimestamp)
	}

	for blockWalker.HasNext() && timestampValid {
		timestampValid = t.checkBlock(blockWalker.Next(), blockWalker, minSupportedTimestamp)
	}
	return timestampValid
}

func (t *TipManager) processBlock(blockID BlockID, blockWalker *walker.Walker[BlockID], markerWalker *walker.Walker[markers.Marker]) {
	t.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		if blockMetadata.StructureDetails() == nil || blockMetadata.StructureDetails().PastMarkers().Size() == 0 {
			// need to walk blocks
			blockWalker.Push(blockID)
			return
		}
		blockMetadata.StructureDetails().PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			if sequenceID == 0 && index == 0 {
				// need to walk blocks
				blockWalker.Push(blockID)
				return false
			}
			pastMarker := markers.NewMarker(sequenceID, index)
			markerWalker.Push(pastMarker)
			return true
		})
	})
}

func (t *TipManager) checkMarker(marker markers.Marker, previousBlockID BlockID, blockWalker *walker.Walker[BlockID], markerWalker *walker.Walker[markers.Marker], minSupportedTimestamp time.Time) (blockID BlockID, timestampValid bool) {
	blockID, blockIssuingTime := t.getMarkerBlock(marker)

	// marker before minSupportedTimestamp
	if blockIssuingTime.Before(minSupportedTimestamp) {
		// marker before minSupportedTimestamp
		if !t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
			// if unconfirmed, then incorrect
			markerWalker.StopWalk()
			return blockID, false
		}
		// if closest past marker is confirmed and before minSupportedTimestamp, then need to walk block past cone of the previously marker block
		blockWalker.Push(previousBlockID)
		return blockID, true
	}
	// confirmed after minSupportedTimestamp
	if t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
		return blockID, true
	}

	// unconfirmed after minSupportedTimestamp

	// check oldest unconfirmed marker time without walking marker DAG
	oldestUnconfirmedMarker := t.getOldestUnconfirmedMarker(marker)

	if timestampValid = t.processMarker(marker, minSupportedTimestamp, oldestUnconfirmedMarker); !timestampValid {
		return
	}

	t.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
		// If there is a confirmed marker before the oldest unconfirmed marker, and it's older than minSupportedTimestamp, need to walk block past cone of oldestUnconfirmedMarker.
		if sequence.LowestIndex() < oldestUnconfirmedMarker.Index() {
			confirmedMarkerIdx := t.getPreviousConfirmedIndex(sequence, oldestUnconfirmedMarker.Index())
			if t.isMarkerOldAndConfirmed(markers.NewMarker(sequence.ID(), confirmedMarkerIdx), minSupportedTimestamp) {
				blockWalker.Push(t.tangle.Booker.MarkersManager.BlockID(oldestUnconfirmedMarker))
			}
		}

		// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
		referencedMarkers := sequence.ReferencedMarkers(marker.Index())
		referencedMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			// Ignore Marker(0, 0) as it sometimes occurs in the past marker cone. Marker mysteries.
			if sequenceID == 0 && index == 0 {
				return true
			}

			referencedMarker := markers.NewMarker(sequenceID, index)
			// if referenced marker is confirmed and older than minSupportedTimestamp, walk unconfirmed block past cone of oldestUnconfirmedMarker
			if t.isMarkerOldAndConfirmed(referencedMarker, minSupportedTimestamp) {
				blockWalker.Push(t.tangle.Booker.MarkersManager.BlockID(oldestUnconfirmedMarker))
				return false
			}
			// otherwise, process the referenced marker
			markerWalker.Push(referencedMarker)
			return true
		})
	})
	return blockID, true
}

// isMarkerOldAndConfirmed check whether previousMarker is confirmed and older than minSupportedTimestamp. It is used to check whether to walk blocks in the past cone of the current marker.
func (t *TipManager) isMarkerOldAndConfirmed(previousMarker markers.Marker, minSupportedTimestamp time.Time) bool {
	referencedMarkerBlkID, referenceMarkerBlkIssuingTime := t.getMarkerBlock(previousMarker)
	if t.tangle.ConfirmationOracle.IsBlockConfirmed(referencedMarkerBlkID) && referenceMarkerBlkIssuingTime.Before(minSupportedTimestamp) {
		return true
	}
	return false
}

func (t *TipManager) processMarker(pastMarker markers.Marker, minSupportedTimestamp time.Time, oldestUnconfirmedMarker markers.Marker) (tscValid bool) {
	// oldest unconfirmed marker is in the future cone of the past marker (same sequence), therefore past marker is confirmed and there is no need to check
	// this condition is covered by other checks but leaving it here just for safety
	if pastMarker.Index() < oldestUnconfirmedMarker.Index() {
		return true
	}
	_, oldestUnconfirmedMarkerBlkIssuingTime := t.getMarkerBlock(oldestUnconfirmedMarker)
	return oldestUnconfirmedMarkerBlkIssuingTime.After(minSupportedTimestamp)
}

func (t *TipManager) checkBlock(blockID BlockID, blockWalker *walker.Walker[BlockID], minSupportedTimestamp time.Time) (timestampValid bool) {
	timestampValid = true

	t.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		// if block is older than TSC then it's incorrect no matter the confirmation status
		if block.IssuingTime().Before(minSupportedTimestamp) {
			timestampValid = false
			blockWalker.StopWalk()
			return
		}

		// if block is younger than TSC and confirmed, then return timestampValid=true
		if t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
			return
		}

		// if block is younger than TSC and not confirmed, walk through strong parents' past cones
		for parentID := range block.ParentsByType(StrongParentType) {
			blockWalker.Push(parentID)
		}
	})
	return timestampValid
}

func (t *TipManager) getMarkerBlock(marker markers.Marker) (markerBlockID BlockID, markerBlockIssuingTime time.Time) {
	if marker.SequenceID() == 0 && marker.Index() == 0 {
		return EmptyBlockID, time.Unix(epoch.GenesisTime, 0)
	}
	blockID := t.tangle.Booker.MarkersManager.BlockID(marker)
	if blockID == EmptyBlockID {
		panic(fmt.Errorf("failed to retrieve marker block for %s", marker.String()))
	}
	t.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		markerBlockID = block.ID()
		markerBlockIssuingTime = block.IssuingTime()
	})

	return
}

// getOldestUnconfirmedMarker is similar to FirstUnconfirmedMarkerIndex, except it skips any marker gaps an existing marker.
func (t *TipManager) getOldestUnconfirmedMarker(pastMarker markers.Marker) markers.Marker {
	unconfirmedMarkerIdx := t.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(pastMarker.SequenceID())

	// skip any gaps in marker indices
	for ; unconfirmedMarkerIdx <= pastMarker.Index(); unconfirmedMarkerIdx++ {
		currentMarker := markers.NewMarker(pastMarker.SequenceID(), unconfirmedMarkerIdx)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if t.tangle.Booker.MarkersManager.BlockID(currentMarker) == EmptyBlockID {
			continue
		}
		break
	}

	oldestUnconfirmedMarker := markers.NewMarker(pastMarker.SequenceID(), unconfirmedMarkerIdx)
	return oldestUnconfirmedMarker
}

func (t *TipManager) getPreviousConfirmedIndex(sequence *markers.Sequence, markerIndex markers.Index) markers.Index {
	// skip any gaps in marker indices
	for ; sequence.LowestIndex() < markerIndex; markerIndex-- {
		currentMarker := markers.NewMarker(sequence.ID(), markerIndex)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap or marker is not yet confirmed (should not be the case).
		if blkID := t.tangle.Booker.MarkersManager.BlockID(currentMarker); blkID == EmptyBlockID || !t.tangle.ConfirmationOracle.IsBlockConfirmed(blkID) {
			continue
		}
		break
	}
	return markerIndex
}

// selectTips returns a list of parents. In case of a transaction, it references young enough attachments
// of consumed transactions directly. Otherwise/additionally count tips are randomly selected.
func (t *TipManager) selectTips(p payload.Payload, count int) (parents BlockIDs) {
	parents = NewBlockIDs()

	tips := t.tips.RandomUniqueEntries(count)

	// only add genesis if no tips are available and not previously referenced (in case of a transaction),
	// or selected ones had incorrect time-since-confirmation
	if len(tips) == 0 {
		parents.Add(EmptyBlockID)
		return
	}

	// at least one tip is returned
	for _, tip := range tips {
		blockID := tip
		if !parents.Contains(blockID) && t.isPastConeTimestampCorrect(blockID) {
			parents.Add(blockID)
		}
	}
	return
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() BlockIDs {
	return retrieveAllTips(t.tips)
}

func retrieveAllTips(tipsMap *randommap.RandomMap[BlockID, BlockID]) BlockIDs {
	mapKeys := tipsMap.Keys()
	tips := NewBlockIDs()

	for _, key := range mapKeys {
		tips.Add(key)
	}
	return tips
}

// TipCount the amount of strong tips.
func (t *TipManager) TipCount() int {
	return t.tips.Size()
}

// Shutdown stops the TipManager.
func (t *TipManager) Shutdown() {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region QueueElement /////////////////////////////////////////////////////////////////////////////////////////////////

// QueueElement is an element in the TimedQueue. It
type QueueElement struct {
	// Value represents the value of the queued element.
	Value BlockID

	// Key represents the time of the element to be used as a key.
	Key time.Time

	index int
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimedHeap ////////////////////////////////////////////////////////////////////////////////////////////////////

type TipsCleaner struct {
	heap       TimedHeap
	tipManager *TipManager
	heapMutex  sync.RWMutex
}

// Add adds a new element to the heap.
func (t *TipsCleaner) Add(key time.Time, value BlockID) {
	t.heapMutex.Lock()
	defer t.heapMutex.Unlock()
	heap.Push(&t.heap, &QueueElement{Value: value, Key: key})
}

// RemoveBefore removes the elements with key time earlier than the given time.
func (t *TipsCleaner) RemoveBefore(minAllowedTime time.Time) {
	t.heapMutex.Lock()
	defer t.heapMutex.Unlock()
	popCounter := 0
	for i := 0; i < t.heap.Len(); i++ {
		if t.heap[i].Key.After(minAllowedTime) {
			break
		}
		popCounter++

	}
	for i := 0; i < popCounter; i++ {
		block := heap.Pop(&t.heap)
		t.tipManager.deleteTip(block.(*QueueElement).Value)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimedHeap ////////////////////////////////////////////////////////////////////////////////////////////////////

// TimedHeap defines a heap based on times.
type TimedHeap []*QueueElement

// Len is the number of elements in the collection.
func (h TimedHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i should sort before the element with index j.
func (h TimedHeap) Less(i, j int) bool {
	return h[i].Key.Before(h[j].Key)
}

// Swap swaps the elements with indexes i and j.
func (h TimedHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

// Push adds x as the last element to the heap.
func (h *TimedHeap) Push(x interface{}) {
	data := x.(*QueueElement)
	*h = append(*h, data)
	data.index = len(*h) - 1
}

// Pop removes and returns the last element of the heap.
func (h *TimedHeap) Pop() interface{} {
	n := len(*h)
	data := (*h)[n-1]
	(*h)[n-1] = nil // avoid memory leak
	*h = (*h)[:n-1]
	data.index = -1
	return data
}

// interface contract (allow the compiler to check if the implementation has all the required methods).
var _ heap.Interface = &TimedHeap{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
