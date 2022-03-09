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
	t.tangle.Dispatcher.Events.MessageDispatched.Attach(events.NewClosure(func(messageID MessageID) {
		t.tangle.Storage.Message(messageID).Consume(t.AddTip)
	}))

	t.Events.TipRemoved.Attach(events.NewClosure(func(tipEvent *TipEvent) {
		t.tipsCleaner.Cancel(tipEvent.MessageID)
	}))

	t.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(events.NewClosure(func(messageID MessageID) {
		t.tangle.Storage.Message(messageID).Consume(t.removeStrongParents)
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

	// Check if any approvers that are confirmed or scheduled and return if true, to guarantee that the parents are not added to the tipset after its approvers.
	if t.checkApprovers(messageID) {
		return
	}
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
	t.removeStrongParents(message)
}

// checkApprovers returns true if the message has any confirmed or scheduled approver.
func (t *TipManager) checkApprovers(messageID MessageID) bool {
	approverScheduledConfirmed := false
	t.tangle.Storage.Approvers(messageID).Consume(func(approver *Approver) {
		if approverScheduledConfirmed {
			return
		}

		approverScheduledConfirmed = t.tangle.ConfirmationOracle.IsMessageConfirmed(approver.approverMessageID)
		if !approverScheduledConfirmed {
			t.tangle.Storage.MessageMetadata(approver.approverMessageID).Consume(func(messageMetadata *MessageMetadata) {
				approverScheduledConfirmed = messageMetadata.Scheduled()
			})
		}
	})
	return approverScheduledConfirmed
}

func (t *TipManager) removeStrongParents(message *Message) {
	message.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) bool {
		if _, deleted := t.tips.Delete(parentMessageID); deleted {
			t.Events.TipRemoved.Trigger(&TipEvent{
				MessageID: parentMessageID,
			})
		}

		return true
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(p payload.Payload, countParents int) (parents MessageIDsSlice, err error) {
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
	}

	return parents, nil
}

func (t *TipManager) isPastConeTimestampCorrect(messageID MessageID) (timestampValid bool) {
	now := clock.SyncedTime()
	minSupportedTimestamp := now.Add(-t.tangle.Options.TimeSinceConfirmationThreshold)
	timestampValid = true

	// skip TSC check if no message has been confirmed to allow attaching to genesis
	if t.tangle.TimeManager.LastConfirmedMessage().MessageID == EmptyMessageID {
		// if the genesis message is the last confirmed message, then there is no point in performing tangle walk
		// return true so that the network can start issuing messages when the tangle starts
		return
	}

	// if last confirmed message if older than minSupportedTimestamp, then all tips are invalid
	if t.tangle.TimeManager.LastConfirmedMessage().Time.Before(minSupportedTimestamp) {
		return false
	}

	if t.tangle.ConfirmationOracle.IsMessageConfirmed(messageID) {
		// selected message is confirmed, therefore it's correct
		return
	}

	// selected message is not confirmed and older than TSC
	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		timestampValid = minSupportedTimestamp.Before(message.IssuingTime())
	})
	if !timestampValid {
		// timestamp of the selected message is invalid
		return
	}

	markerWalker := walker.New(false)
	messageWalker := walker.New(false)

	t.processMessage(messageID, messageWalker, markerWalker)

	for markerWalker.HasNext() && timestampValid {
		marker := markerWalker.Next().(markers.Marker)
		timestampValid = t.checkMarker(&marker, messageWalker, markerWalker, minSupportedTimestamp)
	}

	for messageWalker.HasNext() && timestampValid {
		timestampValid = t.checkMessage(messageWalker.Next().(MessageID), messageWalker, minSupportedTimestamp)
	}
	return timestampValid
}

func (t *TipManager) processMessage(messageID MessageID, messageWalker, markerWalker *walker.Walker) {
	t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		if messageMetadata.StructureDetails() == nil || messageMetadata.StructureDetails().PastMarkers.Size() == 0 {
			// need to walk messages
			messageWalker.Push(messageID)
			return
		}
		messageMetadata.StructureDetails().PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			if sequenceID == 0 && index == 0 {
				// need to walk messages
				messageWalker.Push(messageID)
				return false
			}
			pastMarker := markers.NewMarker(sequenceID, index)
			markerWalker.Push(*pastMarker)
			return true
		})
	})
}

func (t *TipManager) checkMarker(marker *markers.Marker, messageWalker, markerWalker *walker.Walker, minSupportedTimestamp time.Time) (timestampValid bool) {
	message := t.getMarkerMessage(marker)

	if message.IssuingTime().After(minSupportedTimestamp) {
		// confirmed after minSupportedTimestamp
		if t.tangle.ConfirmationOracle.IsMarkerConfirmed(marker) {
			return true
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
				confirmedMarkerIdx := t.getPreviousConfirmedIndex(sequence, oldestUnconfirmedMarker.Index()-1)

				confirmedMarkerMsg := t.getMarkerMessage(markers.NewMarker(sequence.ID(), confirmedMarkerIdx))
				if t.tangle.ConfirmationOracle.IsMessageConfirmed(confirmedMarkerMsg.ID()) && confirmedMarkerMsg.IssuingTime().Before(minSupportedTimestamp) {
					messageWalker.Push(t.tangle.Booker.MarkersManager.MessageID(oldestUnconfirmedMarker))
				}
			}

			// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
			referencedMarkers := sequence.ReferencedMarkers(marker.Index())
			referencedMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
				referencedMarker := markers.NewMarker(sequenceID, index)
				// if referenced marker is confirmed and older than minSupportedTimestamp, walk unconfirmed message past cone of oldestUnconfirmedMarker
				referencedMarkerMsg := t.getMarkerMessage(markers.NewMarker(sequenceID, index))
				if t.tangle.ConfirmationOracle.IsMessageConfirmed(referencedMarkerMsg.ID()) && referencedMarkerMsg.IssuingTime().Before(minSupportedTimestamp) {
					messageWalker.Push(t.tangle.Booker.MarkersManager.MessageID(oldestUnconfirmedMarker))
					return true
				}
				// otherwise, process the referenced marker
				markerWalker.Push(*referencedMarker)
				return true
			})
		})

		return true
	}

	// marker before minSupportedTimestamp
	if !t.tangle.ConfirmationOracle.IsMarkerConfirmed(marker) {
		// if unconfirmed, then incorrect
		markerWalker.StopWalk()
		return false
	}

	// if closest past marker is confirmed and before minSupportedTimestamp, then message should be ok
	return true
}

func (t *TipManager) processMarker(pastMarker *markers.Marker, minSupportedTimestamp time.Time, oldestUnconfirmedMarker *markers.Marker) (tscValid bool) {
	// oldest unconfirmed marker is in the future cone of the past marker (same sequence), therefore past marker is confirmed and there is no need to check
	if pastMarker.Index() < oldestUnconfirmedMarker.Index() {
		return
	}

	unconfirmedMarkerMessage := t.getMarkerMessage(oldestUnconfirmedMarker)
	if unconfirmedMarkerMessage.IssuingTime().Before(minSupportedTimestamp) {
		return false
	}
	return true
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
	})
	return timestampValid
}

func (t *TipManager) getMarkerMessage(marker *markers.Marker) (markerMessage *Message) {
	messageID := t.tangle.Booker.MarkersManager.MessageID(marker)
	if messageID == EmptyMessageID {
		panic(fmt.Errorf("failed to retrieve marker message for %s", marker.String()))
	}
	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		markerMessage = message
	})
	return
}

// getOldestUnconfirmedMarker is similar to FirstUnconfirmedMarkerIndex, except it skips any marker gaps an existing marker.
func (t *TipManager) getOldestUnconfirmedMarker(pastMarker *markers.Marker) *markers.Marker {
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
	for ; sequence.LowestIndex() <= markerIndex; markerIndex-- {
		currentMarker := markers.NewMarker(sequence.ID(), markerIndex)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap or marker is not yet confirmed (should not be the case).
		if t.tangle.Booker.MarkersManager.MessageID(currentMarker) == EmptyMessageID || !t.tangle.ConfirmationOracle.IsMarkerConfirmed(currentMarker) {
			continue
		}
		break
	}
	return markerIndex
}

// selectTips returns a list of parents. In case of a transaction, it references young enough attachments
// of consumed transactions directly. Otherwise/additionally count tips are randomly selected.
func (t *TipManager) selectTips(p payload.Payload, count int) (parents MessageIDsSlice) {
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

	// only add genesis if no tips are available and not previously referenced (in case of a transaction),
	// or selected ones had incorrect time-since-confirmation
	if len(parents) == 0 && len(tips) == 0 {
		parents = append(parents, EmptyMessageID)
	}

	// at least one tip is returned
	for _, tip := range tips {
		messageID := tip.(MessageID)
		if _, ok := parentsMap[messageID]; !ok && t.isPastConeTimestampCorrect(messageID) {
			parentsMap[messageID] = types.Void
			parents = append(parents, messageID)
		}
	}

	return
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() MessageIDsSlice {
	return retrieveAllTips(t.tips)
}

func retrieveAllTips(tipsMap *randommap.RandomMap) MessageIDsSlice {
	mapKeys := tipsMap.Keys()
	tips := make(MessageIDsSlice, len(mapKeys))
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
