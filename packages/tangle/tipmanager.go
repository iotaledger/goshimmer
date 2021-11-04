package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/randommap"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timedexecutor"
	"github.com/iotaledger/hive.go/timedqueue"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region TimedTaskExecutor ////////////////////////////////////////////////////////////////////////////////////////////

// TimedTaskExecutor is a TimedExecutor that internally manages the scheduled callbacks as tasks with a unique
// identifier. It allows to replace existing scheduled tasks and cancel them using the same identifier.
type TimedTaskExecutor struct {
	*timedexecutor.TimedExecutor
	queuedElements      map[interface{}]*timedqueue.QueueElement
	queuedElementsMutex sync.Mutex
}

// NewTimedTaskExecutor is the constructor of the TimedTaskExecutor.
func NewTimedTaskExecutor(workerCount int) *TimedTaskExecutor {
	return &TimedTaskExecutor{
		TimedExecutor:  timedexecutor.New(workerCount),
		queuedElements: make(map[interface{}]*timedqueue.QueueElement),
	}
}

// ExecuteAfter executes the given function after the given delay.
func (t *TimedTaskExecutor) ExecuteAfter(identifier interface{}, callback func(), delay time.Duration) *timedexecutor.ScheduledTask {
	t.queuedElementsMutex.Lock()
	defer t.queuedElementsMutex.Unlock()

	queuedElement, queuedElementExists := t.queuedElements[identifier]
	if queuedElementExists {
		queuedElement.Cancel()
	}

	t.queuedElements[identifier] = t.TimedExecutor.ExecuteAfter(func() {
		callback()

		t.queuedElementsMutex.Lock()
		defer t.queuedElementsMutex.Unlock()

		delete(t.queuedElements, identifier)
	}, delay)

	return t.queuedElements[identifier]
}

// ExecuteAt executes the given function at the given time.
func (t *TimedTaskExecutor) ExecuteAt(identifier interface{}, callback func(), executionTime time.Time) *timedexecutor.ScheduledTask {
	t.queuedElementsMutex.Lock()
	defer t.queuedElementsMutex.Unlock()

	queuedElement, queuedElementExists := t.queuedElements[identifier]
	if queuedElementExists {
		queuedElement.Cancel()
	}

	t.queuedElements[identifier] = t.TimedExecutor.ExecuteAt(func() {
		callback()

		t.queuedElementsMutex.Lock()
		defer t.queuedElementsMutex.Unlock()

		delete(t.queuedElements, identifier)
	}, executionTime)

	return t.queuedElements[identifier]
}

// Cancel cancels a queued task.
func (t *TimedTaskExecutor) Cancel(identifier interface{}) (canceled bool) {
	t.queuedElementsMutex.Lock()
	defer t.queuedElementsMutex.Unlock()

	queuedElement, queuedElementExists := t.queuedElements[identifier]
	if !queuedElementExists {
		return
	}

	queuedElement.Cancel()
	delete(t.queuedElements, identifier)

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

const tipLifeGracePeriod = maxParentsTimeDifference - 1*time.Minute

// TipManager manages a map of tips and emits events for their removal and addition.
type TipManager struct {
	tangle      *Tangle
	tips        *randommap.RandomMap
	tipsCleaner *TimedTaskExecutor
	Events      *TipManagerEvents
}

// NewTipManager creates a new tip-selector.
func NewTipManager(tangle *Tangle, tips ...MessageID) *TipManager {
	tipSelector := &TipManager{
		tangle:      tangle,
		tips:        randommap.New(),
		tipsCleaner: NewTimedTaskExecutor(1),
		Events: &TipManagerEvents{
			TipAdded:   events.NewEvent(tipEventHandler),
			TipRemoved: events.NewEvent(tipEventHandler),
		},
	}

	if tips != nil {
		tipSelector.Set(tips...)
	}

	return tipSelector
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TipManager) Setup() {
	t.tangle.Orderer.Events.MessageOrdered.Attach(events.NewClosure(func(messageID MessageID) {
		t.tangle.Storage.Message(messageID).Consume(t.AddTip)
	}))

	t.Events.TipRemoved.Attach(events.NewClosure(func(tipEvent *TipEvent) {
		t.tipsCleaner.Cancel(tipEvent.MessageID)
	}))
}

// Set adds the given messageIDs as tips.
func (t *TipManager) Set(tips ...MessageID) {
	for _, messageID := range tips {
		t.tips.Set(messageID, messageID)
	}
}

// AddTip adds the message to the tip pool if its issuing time is within the tipLifeGracePeriod.
// Parents of a message that are currently tip lose the tip status and are removed.
func (t *TipManager) AddTip(message *Message) {
	messageID := message.ID()
	cachedMessageMetadata := t.tangle.Storage.MessageMetadata(messageID)
	messageMetadata := cachedMessageMetadata.Unwrap()
	defer cachedMessageMetadata.Release()

	if messageMetadata == nil {
		panic(fmt.Errorf("failed to load MessageMetadata with %s", messageID))
	}

	if clock.Since(message.IssuingTime()) > tipLifeGracePeriod {
		return
	}

	// TODO: possible logical race condition if a child message gets added before its parents.
	//  To be sure we probably need to check "It is not directly referenced by any strong message via strong/weak parent"
	//  before adding a message as a tip. For now we're using only 1 worker after the scheduler and it shouldn't be a problem.

	if t.tips.Set(messageID, messageID) {
		t.Events.TipAdded.Trigger(&TipEvent{
			MessageID: messageID,
		})

		t.tipsCleaner.ExecuteAt(messageID, func() {
			t.tips.Delete(messageID)
		}, message.IssuingTime().Add(tipLifeGracePeriod))
	}

	// skip removing tips if TangleWidth is enabled
	if t.TipCount() <= t.tangle.Options.TangleWidth {
		return
	}

	// a tip loses its tip status if it is referenced by another message
	message.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) {
		if _, deleted := t.tips.Delete(parentMessageID); deleted {
			t.Events.TipRemoved.Trigger(&TipEvent{
				MessageID: parentMessageID,
			})
		}
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(p payload.Payload, countParents int) (parents MessageIDs, err error) {
	if countParents > MaxParentsCount {
		countParents = MaxParentsCount
	}
	if countParents < MinParentsCount {
		countParents = MinParentsCount
	}

	// select parents
	parents = t.selectTips(p, countParents)
	// if transaction, make sure that all inputs are in the past cone of the selected tips
	if p != nil && p.Type() == ledgerstate.TransactionType {
		transaction := p.(*ledgerstate.Transaction)

		tries := 5
		for !t.tangle.Utils.AllTransactionsApprovedByMessages(transaction.ReferencedTransactionIDs(), parents...) || len(parents) == 0 {
			if tries == 0 {
				err = errors.Errorf("not able to make sure that all inputs are in the past cone of selected tips and parents have correct time-since-confirmation")
				return nil, err
			}
			tries--

			parents = t.selectTips(p, MaxParentsCount)
		}
	} else {
		tries := 5
		for len(parents) == 0 {
			if tries == 0 {
				err = errors.Errorf("not able to make sure that parents have correct time-since-confirmation")
				return nil, err
			}
			tries--

			parents = t.selectTips(p, MaxParentsCount)
		}
	}
	return parents, nil
}

func (t *TipManager) isPastConeTimestampCorrect(messageID MessageID) (timestampValid bool) {
	now := clock.SyncedTime()
	markersVisited := 0
	messagesVisited := 0
	defer func() {
		fmt.Println("Checking past cone took ", clock.Since(now).String(), "markers visited:", markersVisited, "messagesVisited", messagesVisited)
	}()
	minSupportedTimestamp := now.Add(-oldMessageThreshold)
	timestampValid = true

	if t.tangle.TimeManager.LastConfirmedMessage().MessageID == EmptyMessageID {
		// if the genesis message is the last confirmed message, then there is no point in performing tangle walk
		// return true so that the network can start issuing messages when the tangle starts
		return
	}

	if t.tangle.TimeManager.Time().Before(minSupportedTimestamp) {
		// FIXME: this opens a hole to confirm incorrect timestamps if the oldest supported timestamp is bigger than current tangleTime
		// possibly it should be required that timestampValidThreshold > syncThreshold
		return false
	}

	if t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
		// selected message is confirmed, therefore it's correct
		return
	}

	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		timestampValid = message.IssuingTime().After(minSupportedTimestamp)
	})
	if !timestampValid {
		// timestamp of the selected message is invalid
		return
	}

	markerWalker := walker.New(false)
	messageWalker := walker.New(false)

	t.processMessage(messageID, messageWalker, markerWalker, minSupportedTimestamp, true)

	for markerWalker.HasNext() && timestampValid {
		markersVisited++
		timestampValid = t.checkMarker(markerWalker.Next().(markers.Marker), messageWalker, markerWalker, minSupportedTimestamp)
	}

	for messageWalker.HasNext() && timestampValid {
		messagesVisited++
		timestampValid = t.checkMessage(messageWalker.Next().(MessageID), messageWalker, minSupportedTimestamp)
	}
	return timestampValid
}

func (t *TipManager) checkMarker(marker markers.Marker, messageWalker, markerWalker *walker.Walker, minSupportedTimestamp time.Time) (timestampValid bool) {
	message := t.getMarkerMessage(marker)

	if message.IssuingTime().After(minSupportedTimestamp) {
		// confirmed after minSupportedTimestamp
		if t.tangle.ConfirmationOracle.IsMarkerConfirmed(&marker) {
			return true
		}

		// unconfirmed after minSupportedTimestamp
		t.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
			foundExistingMarker := false
			prevIndex := marker.Index() - 1
			var previousPastMarker *markers.Marker
			var prevReferencedMarkers *markers.Markers
			for !foundExistingMarker && sequence.LowestIndex() <= prevIndex {
				previousPastMarker = markers.NewMarker(marker.SequenceID(), prevIndex)
				// Skip if there is no marker at the given index, i.e., the sequence has a gap.
				foundExistingMarker = t.tangle.Booker.MarkersManager.MessageID(previousPastMarker) != EmptyMessageID
				if !foundExistingMarker {
					prevIndex--
					continue
				}
				t.processMarker(previousPastMarker, messageWalker, markerWalker, message.ID(), minSupportedTimestamp)
				prevReferencedMarkers = sequence.ReferencedMarkers(previousPastMarker.Index())
			}
			referencedMarkers := sequence.ReferencedMarkers(marker.Index())
			referencedMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
				referencedMarker := markers.NewMarker(sequenceID, index)

				if sequenceID == 0 {
					// walk past cone of the message in sequence 0
					if index != 0 {
						messageWalker.Push(t.tangle.Booker.MarkersManager.MessageID(referencedMarker))
					}
					return true
				}
				if previousPastMarker == nil {
					t.processMarker(referencedMarker, messageWalker, markerWalker, message.ID(), minSupportedTimestamp)
					return true
				}
				// do not process referenced marker if previous marker in the same sequence also references it
				if previousReferencedIndex, exists := prevReferencedMarkers.Get(sequenceID); !exists || previousReferencedIndex > index {
					t.processMarker(referencedMarker, messageWalker, markerWalker, message.ID(), minSupportedTimestamp)
				}
				return true
			})
		})
		return true
	}

	// marker before minSupportedTimestamp
	if !t.tangle.ConfirmationOracle.IsMarkerConfirmed(&marker) {
		// if unconfirmed, then incorrect
		markerWalker.StopWalk()
		return false
	}

	// if confirmed, then need to walk past cone of previous marker message, which is impossible here
	// should not reach here as the case of unconfirmed marker older than confirmation window should be handled without adding it to the walker
	return false
}

func (t *TipManager) checkMessage(messageID MessageID, messageWalker *walker.Walker, minSupportedTimestamp time.Time) (timestampValid bool) {
	timestampValid = true
	if t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
		return
	}

	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if message.IssuingTime().Before(minSupportedTimestamp) {
			timestampValid = false
			messageWalker.StopWalk()
			return
		}

		// walk through strong parents' past cones
		for _, parentID := range message.ParentsByType(StrongParentType) {
			messageWalker.Push(parentID)
		}
		// walk through like parents' past cones
		for _, parentID := range message.ParentsByType(LikeParentType) {
			messageWalker.Push(parentID)
		}
	})
	return
}

func (t *TipManager) processMarker(pastMarker *markers.Marker, messageWalker, markerWalker *walker.Walker, messageID MessageID, minSupportedTimestamp time.Time) {
	pastMarkerMessage := t.getMarkerMessage(*pastMarker)

	if t.tangle.ConfirmationOracle.IsMarkerConfirmed(pastMarker) && pastMarkerMessage.IssuingTime().Before(minSupportedTimestamp) {
		// if referenced marker is confirmed, but is before minSupportedTimestamp, then walk messages in current marker's past cone
		messageWalker.Push(messageID)
		return
	}

	if t.tangle.ConfirmationOracle.IsMarkerConfirmed(pastMarker) && !pastMarkerMessage.IssuingTime().Before(minSupportedTimestamp) {
		// if referenced marker is confirmed and is after minSupportedTimestamp, then it's correct
		return
	}

	// add unconfirmed marker to the walker
	markerWalker.Push(*pastMarker)

}

func (t *TipManager) processMessage(messageID MessageID, messageWalker, markerWalker *walker.Walker, minSupportedTimestamp time.Time, processMarkers bool) (success bool) {
	success = t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		if messageMetadata.StructureDetails() == nil || messageMetadata.StructureDetails().PastMarkers.Size() == 0 || true {
			// need to walk messages
			messageWalker.Push(messageID)
			return
		}
		messageMetadata.StructureDetails().PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			// Avoid adding and tracking support of markers in sequence 0.
			if sequenceID == 0 {
				if index != 0 {
					messageWalker.Push(messageID)
				}
				return true
			}
			if processMarkers {
				pastMarker := markers.NewMarker(sequenceID, index)
				t.processMarker(pastMarker, messageWalker, markerWalker, messageID, minSupportedTimestamp)
			}
			return true
		})
	})

	return
}

func (t *TipManager) getMarkerMessage(marker markers.Marker) (markerMessage *Message) {
	if !t.tangle.Storage.MarkerMessageMapping(&marker).Consume(func(markerMessageMapping *MarkerMessageMapping) {
		t.tangle.Storage.Message(markerMessageMapping.MessageID()).Consume(func(message *Message) {
			markerMessage = message
		})
	}) {
		panic(fmt.Errorf("failed to retrieve marker message for %s", marker.String()))
	}
	return
}

// selectTips returns a list of parents. In case of a transaction, it references young enough attachments
// of consumed transactions directly. Otherwise/additionally count tips are randomly selected.
func (t *TipManager) selectTips(p payload.Payload, count int) (parents MessageIDs) {
	parents = make([]MessageID, 0, MaxParentsCount)
	parentsMap := make(map[MessageID]types.Empty)

	// if transaction: reference young parents directly
	if p != nil && p.Type() == ledgerstate.TransactionType {
		transaction := p.(*ledgerstate.Transaction)

		referencedTransactionIDs := transaction.ReferencedTransactionIDs()
		if len(referencedTransactionIDs) <= 8 {
			for transactionID := range referencedTransactionIDs {
				// only one attachment needs to be added
				added := false

				for _, attachmentMessageID := range t.tangle.Storage.AttachmentMessageIDs(transactionID) {
					t.tangle.Storage.Message(attachmentMessageID).Consume(func(message *Message) {
						// check if message is too old
						timeDifference := clock.SyncedTime().Sub(message.IssuingTime())
						if timeDifference <= maxParentsTimeDifference && t.isPastConeTimestampCorrect(attachmentMessageID) {
							if _, ok := parentsMap[attachmentMessageID]; !ok {
								parentsMap[attachmentMessageID] = types.Void
								parents = append(parents, attachmentMessageID)
							}
							added = true
						}
					})

					if added {
						break
					}
				}
			}
		} else {
			// if there are more than 8 referenced transactions:
			// for now we simply select as many parents as possible and hope all transactions will be covered
			count = MaxParentsCount
		}
	}

	// nothing to do anymore
	if len(parents) == MaxParentsCount {
		return
	}

	// select some current tips (depending on length of parents)
	if count+len(parents) > MaxParentsCount {
		count = MaxParentsCount - len(parents)
	}

	tips := t.tips.RandomUniqueEntries(count)

	// at least one tip is returned
	for _, tip := range tips {
		messageID := tip.(MessageID)
		if !t.isPastConeTimestampCorrect(messageID) {
			// remove tip with incorrect timestamp in the past cone
			t.tips.Delete(messageID)
		} else if _, ok := parentsMap[messageID]; !ok {
			parentsMap[messageID] = types.Void
			parents = append(parents, messageID)
		}
	}

	// only add genesis if no tips are available and not previously referenced (in case of a transaction),
	// or selected ones had incorrect time-since-confirmation
	if len(parents) == 0 && t.tips.Size() == 0 {
		parents = append(parents, EmptyMessageID)
	}
	return
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() MessageIDs {
	return retrieveAllTips(t.tips)
}

func retrieveAllTips(tipsMap *randommap.RandomMap) MessageIDs {
	mapKeys := tipsMap.Keys()
	tips := make(MessageIDs, len(mapKeys))
	for i, key := range mapKeys {
		tips[i] = key.(MessageID)
	}
	return tips
}

// TipCount the amount of strong tips.
func (t *TipManager) TipCount() int {
	return t.tips.Size()
}

// Shutdown stops the TipManager.
func (t *TipManager) Shutdown() {
	t.tipsCleaner.Shutdown(timedexecutor.CancelPendingTasks)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipManagerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// TipManagerEvents represents events happening on the TipManager.
type TipManagerEvents struct {
	// Fired when a tip is added.
	TipAdded *events.Event

	// Fired when a tip is removed.
	TipRemoved *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipEvent /////////////////////////////////////////////////////////////////////////////////////////////////////

// TipEvent holds the information provided by the TipEvent event that gets triggered when a message gets added or
// removed as tip.
type TipEvent struct {
	// MessageID of the added/removed tip.
	MessageID MessageID
}

func tipEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(event *TipEvent))(params[0].(*TipEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
