package tangle

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/randommap"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

const tipLifeGracePeriod = maxParentsTimeDifference - 1*time.Minute

// TipManager manages a map of tips and emits events for their removal and addition.
type TipManager struct {
	tangle              *Tangle
	tips                *randommap.RandomMap[MessageID, MessageID]
	tipsCleaner         *TipsCleaner
	tipsConflictTracker *TipsConflictTracker
	Events              *TipManagerEvents
}

// NewTipManager creates a new tip-selector.
func NewTipManager(tangle *Tangle, tips ...MessageID) *TipManager {
	tipManager := &TipManager{
		tangle:              tangle,
		tips:                randommap.New[MessageID, MessageID](),
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
	t.tangle.Scheduler.Events.MessageScheduled.Attach(event.NewClosure(func(event *MessageScheduledEvent) {
		t.tangle.Storage.Message(event.MessageID).Consume(t.AddTip)
	}))

	t.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *MessageConfirmedEvent) {
		t.removeStrongParents(event.Message)
	}))

	t.tangle.OrphanageManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
		t.deleteTip(event.Block.ID())
	}))

	t.tangle.TimeManager.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *TimeUpdate) {
		t.tipsCleaner.RemoveBefore(event.NewTime.Add(-t.tangle.Options.TimeSinceConfirmationThreshold))
	}))

	t.tangle.OrphanageManager.Events.AllChildrenOrphaned.Hook(event.NewClosure(func(block *Message) {
		if clock.Since(block.IssuingTime()) > tipLifeGracePeriod {
			return
		}

		t.addTip(block)
	}))

	t.tipsConflictTracker.Setup()
}

// set adds the given messageIDs as tips.
func (t *TipManager) set(tips ...MessageID) {
	for _, messageID := range tips {
		t.tips.Set(messageID, messageID)
	}
}

// AddTip adds the message to the tip pool if its issuing time is within the tipLifeGracePeriod.
// Parents of a message that are currently tip lose the tip status and are removed.
func (t *TipManager) AddTip(message *Message) {
	messageID := message.ID()

	if clock.Since(message.IssuingTime()) > tipLifeGracePeriod {
		return
	}

	// Check if any approvers that are confirmed or scheduled and return if true, to guarantee that the parents are not added to the tipset after its approvers.
	if t.checkApprovers(messageID) {
		return
	}

	if !t.addTip(message) {
		return
	}

	// skip removing tips if TangleWidth is enabled
	if t.TipCount() <= t.tangle.Options.TangleWidth {
		return
	}

	// a tip loses its tip status if it is referenced by another message
	t.removeStrongParents(message)
}

func (t *TipManager) addTip(message *Message) (added bool) {
	messageID := message.ID()

	var invalid bool
	t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		invalid = messageMetadata.IsSubjectivelyInvalid()
	})
	if invalid {
		// fmt.Println("TipManager: skipping adding tip because it is subjectively invalid", messageID)
		return false
	}

	if t.tips.Set(messageID, messageID) {
		t.tipsConflictTracker.AddTip(messageID)
		t.Events.TipAdded.Trigger(&TipEvent{
			MessageID: messageID,
		})

		t.tipsCleaner.Add(message.IssuingTime(), messageID)
		return true
	}

	return false
}

func (t *TipManager) deleteTip(msgID MessageID) (deleted bool) {
	if _, deleted = t.tips.Delete(msgID); deleted {
		t.tipsConflictTracker.RemoveTip(msgID)
		t.Events.TipRemoved.Trigger(&TipEvent{
			MessageID: msgID,
		})
	}
	return
}

// checkApprovers returns true if the message has any confirmed or scheduled approver.
func (t *TipManager) checkApprovers(messageID MessageID) bool {
	approverScheduledConfirmed := false
	t.tangle.Storage.Approvers(messageID).Consume(func(approver *Approver) {
		if approverScheduledConfirmed {
			return
		}

		approverScheduledConfirmed = t.tangle.ConfirmationOracle.IsMessageConfirmed(approver.ApproverMessageID())
		if !approverScheduledConfirmed {
			t.tangle.Storage.MessageMetadata(approver.ApproverMessageID()).Consume(func(messageMetadata *MessageMetadata) {
				approverScheduledConfirmed = messageMetadata.Scheduled()
			})
		}
	})
	return approverScheduledConfirmed
}

func (t *TipManager) removeStrongParents(message *Message) {
	message.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) bool {
		// We do not want to remove the tip if it is the last one representing a pending branch.
		// if t.isLastTipForBranch(parentMessageID) {
		// 	return true
		// }

		t.deleteTip(parentMessageID)

		return true
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(p payload.Payload, countParents int) (parents MessageIDs) {
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
func (t *TipManager) isPastConeTimestampCorrect(messageID MessageID) (timestampValid bool) {
	minSupportedTimestamp := t.tangle.TimeManager.ATT().Add(-t.tangle.Options.TimeSinceConfirmationThreshold)
	timestampValid = true

	// skip TSC check if no message has been confirmed to allow attaching to genesis
	if t.tangle.TimeManager.LastAcceptedMessage().MessageID == EmptyMessageID {
		// if the genesis message is the last confirmed message, then there is no point in performing tangle walk
		// return true so that the network can start issuing messages when the tangle starts
		return true
	}

	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		timestampValid = minSupportedTimestamp.Before(message.IssuingTime())
	})

	if !timestampValid {
		return false
	}
	if t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
		// return true if message is confirmed and has valid timestamp
		return true
	}

	markerWalker := walker.New[markers.Marker](false)
	messageWalker := walker.New[MessageID](false)

	t.processMessage(messageID, messageWalker, markerWalker)
	previousMessageID := messageID
	for markerWalker.HasNext() && timestampValid {
		marker := markerWalker.Next()
		previousMessageID, timestampValid = t.checkMarker(marker, previousMessageID, messageWalker, markerWalker, minSupportedTimestamp)
	}

	for messageWalker.HasNext() && timestampValid {
		timestampValid = t.checkMessage(messageWalker.Next(), messageWalker, minSupportedTimestamp)
	}
	return timestampValid
}

func (t *TipManager) processMessage(messageID MessageID, messageWalker *walker.Walker[MessageID], markerWalker *walker.Walker[markers.Marker]) {
	t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		if messageMetadata.StructureDetails() == nil || messageMetadata.StructureDetails().PastMarkers().Size() == 0 {
			// need to walk messages
			messageWalker.Push(messageID)
			return
		}
		messageMetadata.StructureDetails().PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			if sequenceID == 0 && index == 0 {
				// need to walk messages
				messageWalker.Push(messageID)
				return false
			}
			pastMarker := markers.NewMarker(sequenceID, index)
			markerWalker.Push(pastMarker)
			return true
		})
	})
}

func (t *TipManager) checkMarker(marker markers.Marker, previousMessageID MessageID, messageWalker *walker.Walker[MessageID], markerWalker *walker.Walker[markers.Marker], minSupportedTimestamp time.Time) (messageID MessageID, timestampValid bool) {
	messageID, messageIssuingTime := t.getMarkerMessage(marker)

	// marker before minSupportedTimestamp
	if messageIssuingTime.Before(minSupportedTimestamp) {
		// marker before minSupportedTimestamp
		if !t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
			// if unconfirmed, then incorrect
			markerWalker.StopWalk()
			return messageID, false
		}
		// if closest past marker is confirmed and before minSupportedTimestamp, then need to walk message past cone of the previously marker message
		messageWalker.Push(previousMessageID)
		return messageID, true
	}
	// confirmed after minSupportedTimestamp
	if t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
		return messageID, true
	}

	// unconfirmed after minSupportedTimestamp

	// check oldest unconfirmed marker time without walking marker DAG
	oldestUnconfirmedMarker := t.getOldestUnconfirmedMarker(marker)

	if timestampValid = t.processMarker(marker, minSupportedTimestamp, oldestUnconfirmedMarker); !timestampValid {
		return
	}

	t.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
		// If there is a confirmed marker before the oldest unconfirmed marker, and it's older than minSupportedTimestamp, need to walk message past cone of oldestUnconfirmedMarker.
		if sequence.LowestIndex() < oldestUnconfirmedMarker.Index() {
			confirmedMarkerIdx := t.getPreviousConfirmedIndex(sequence, oldestUnconfirmedMarker.Index())
			if t.isMarkerOldAndConfirmed(markers.NewMarker(sequence.ID(), confirmedMarkerIdx), minSupportedTimestamp) {
				messageWalker.Push(t.tangle.Booker.MarkersManager.MessageID(oldestUnconfirmedMarker))
			}
		}

		// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
		referencedMarkers := sequence.ReferencedMarkers(marker.Index())
		referencedMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			referencedMarker := markers.NewMarker(sequenceID, index)
			// if referenced marker is confirmed and older than minSupportedTimestamp, walk unconfirmed message past cone of oldestUnconfirmedMarker
			if t.isMarkerOldAndConfirmed(referencedMarker, minSupportedTimestamp) {
				messageWalker.Push(t.tangle.Booker.MarkersManager.MessageID(oldestUnconfirmedMarker))
				return false
			}
			// otherwise, process the referenced marker
			markerWalker.Push(referencedMarker)
			return true
		})
	})
	return messageID, true
}

// isMarkerOldAndConfirmed check whether previousMarker is confirmed and older than minSupportedTimestamp. It is used to check whether to walk messages in the past cone of the current marker.
func (t *TipManager) isMarkerOldAndConfirmed(previousMarker markers.Marker, minSupportedTimestamp time.Time) bool {
	referencedMarkerMsgID, referenceMarkerMsgIssuingTime := t.getMarkerMessage(previousMarker)
	if t.tangle.ConfirmationOracle.IsMessageConfirmed(referencedMarkerMsgID) && referenceMarkerMsgIssuingTime.Before(minSupportedTimestamp) {
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
	_, oldestUnconfirmedMarkerMsgIssuingTime := t.getMarkerMessage(oldestUnconfirmedMarker)
	return oldestUnconfirmedMarkerMsgIssuingTime.After(minSupportedTimestamp)
}

func (t *TipManager) checkMessage(messageID MessageID, messageWalker *walker.Walker[MessageID], minSupportedTimestamp time.Time) (timestampValid bool) {
	timestampValid = true

	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		// if message is older than TSC then it's incorrect no matter the confirmation status
		if message.IssuingTime().Before(minSupportedTimestamp) {
			timestampValid = false
			messageWalker.StopWalk()
			return
		}

		// if message is younger than TSC and confirmed, then return timestampValid=true
		if t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
			return
		}

		// if message is younger than TSC and not confirmed, walk through strong parents' past cones
		for parentID := range message.ParentsByType(StrongParentType) {
			messageWalker.Push(parentID)
		}
	})
	return timestampValid
}

func (t *TipManager) getMarkerMessage(marker markers.Marker) (markerMessageID MessageID, markerMessageIssuingTime time.Time) {
	if marker.SequenceID() == 0 && marker.Index() == 0 {
		return EmptyMessageID, time.Unix(DefaultGenesisTime, 0)
	}
	messageID := t.tangle.Booker.MarkersManager.MessageID(marker)
	if messageID == EmptyMessageID {
		panic(fmt.Errorf("failed to retrieve marker message for %s", marker.String()))
	}
	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		markerMessageID = message.ID()
		markerMessageIssuingTime = message.IssuingTime()
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
		if t.tangle.Booker.MarkersManager.MessageID(currentMarker) == EmptyMessageID {
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
		if msgID := t.tangle.Booker.MarkersManager.MessageID(currentMarker); msgID == EmptyMessageID || !t.tangle.ConfirmationOracle.IsMessageConfirmed(msgID) {
			continue
		}
		break
	}
	return markerIndex
}

// selectTips returns a list of parents. In case of a transaction, it references young enough attachments
// of consumed transactions directly. Otherwise/additionally count tips are randomly selected.
func (t *TipManager) selectTips(p payload.Payload, count int) (parents MessageIDs) {
	parents = NewMessageIDs()

	tips := t.tips.RandomUniqueEntries(count)

	// only add genesis if no tips are available and not previously referenced (in case of a transaction),
	// or selected ones had incorrect time-since-confirmation
	if len(tips) == 0 {
		parents.Add(EmptyMessageID)
		return
	}

	// at least one tip is returned
	for _, tip := range tips {
		messageID := tip
		if !parents.Contains(messageID) && t.isPastConeTimestampCorrect(messageID) {
			parents.Add(messageID)
		}
	}
	return
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() MessageIDs {
	return retrieveAllTips(t.tips)
}

func retrieveAllTips(tipsMap *randommap.RandomMap[MessageID, MessageID]) MessageIDs {
	mapKeys := tipsMap.Keys()
	tips := NewMessageIDs()

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
	Value MessageID

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
func (t *TipsCleaner) Add(key time.Time, value MessageID) {
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
		message := heap.Pop(&t.heap)
		t.tipManager.deleteTip(message.(*QueueElement).Value)
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
