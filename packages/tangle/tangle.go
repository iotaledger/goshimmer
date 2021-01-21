package tangle

import (
	"container/list"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
)

// Tangle represents the base layer of messages.
type Tangle struct {
	*MessageStore

	Events *Events

	solidifierWorkerPool async.WorkerPool
}

// New creates a new Tangle.
func New(store kvstore.KVStore) (result *Tangle) {
	result = &Tangle{
		MessageStore: NewMessageStore(store),
		Events:       newEvents(),
	}

	result.solidifierWorkerPool.Tune(1024)
	return
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() *Tangle {
	t.solidifierWorkerPool.ShutdownGracefully()
	t.MessageStore.Shutdown()

	return t
}

// worker that stores the message and calls the corresponding storage events.
func (t *Tangle) SolidifyMessage(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {
	// check message solidity
	t.solidifierWorkerPool.Submit(func() {
		t.checkMessageSolidityAndPropagate(cachedMessage, cachedMsgMetadata)
	})
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (t *Tangle) isMessageMarkedAsSolid(messageID MessageID) bool {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessageMetadata and trigger the MessageMissing event if it doesn't exist
	msgMetadataCached := t.StoreIfMissingMessageMetadata(messageID)
	defer msgMetadataCached.Release()

	// return false if the metadata does not exist
	msgMetadata := msgMetadataCached.Unwrap()
	if msgMetadata == nil {
		return false
	}

	// return the solid flag of the metadata object
	return msgMetadata.IsSolid()
}

// checks whether the given message is solid by examining whether its parent1 and
// parent2 messages are solid.
func (t *Tangle) isMessageSolid(msg *Message, msgMetadata *MessageMetadata) bool {
	if msg == nil || msg.IsDeleted() {
		return false
	}

	if msgMetadata == nil || msgMetadata.IsDeleted() {
		return false
	}

	if msgMetadata.IsSolid() {
		return true
	}

	solid := true

	msg.ForEachParent(func(parent Parent) {
		// as missing messages are requested in isMessageMarkedAsSolid,
		// we want to prevent short-circuit evaluation, thus we need to use a tmp variable
		// to avoid side effects from comparing directly to the function call.
		tmp := t.isMessageMarkedAsSolid(parent.ID)
		solid = solid && tmp
	})

	return solid
}

// builds up a stack from the given message and tries to solidify into the present.
// missing messages which are needed for a message to become solid are marked as missing.
func (t *Tangle) checkMessageSolidityAndPropagate(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {

	popElementsFromStack := func(stack *list.List) (*CachedMessage, *CachedMessageMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedMsg := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedMsgMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)
		return currentCachedMsg.(*CachedMessage), currentCachedMsgMetadata.(*CachedMessageMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([2]interface{}{cachedMessage, cachedMsgMetadata})

	// processed messages that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentCachedMessage, currentCachedMsgMetadata := popElementsFromStack(solidificationStack)

		currentMessage := currentCachedMessage.Unwrap()
		currentMsgMetadata := currentCachedMsgMetadata.Unwrap()
		if currentMessage == nil || currentMsgMetadata == nil {
			currentCachedMessage.Release()
			currentCachedMsgMetadata.Release()
			continue
		}

		// mark the message as solid if it has become solid
		if t.isMessageSolid(currentMessage, currentMsgMetadata) && currentMsgMetadata.SetSolid(true) {
			t.Events.MessageSolid.Trigger(&CachedMessageEvent{
				Message:         currentCachedMessage,
				MessageMetadata: currentCachedMsgMetadata})

			// auto. push approvers of the newly solid message to propagate solidification
			t.Approvers(currentMessage.ID()).Consume(func(approver *Approver) {
				approverMessageID := approver.ApproverMessageID()
				solidificationStack.PushBack([2]interface{}{
					t.Message(approverMessageID),
					t.MessageMetadata(approverMessageID),
				})
			})
		}

		currentCachedMessage.Release()
		currentCachedMsgMetadata.Release()
	}
}

// deletes a message and its future cone of messages/approvers.
// nolint
func (t *Tangle) deleteFutureCone(messageID MessageID) {
	cleanupStack := list.New()
	cleanupStack.PushBack(messageID)

	processedMessages := make(map[MessageID]types.Empty)
	processedMessages[messageID] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentMessageID := currentStackEntry.Value.(MessageID)
		cleanupStack.Remove(currentStackEntry)

		t.DeleteMessage(currentMessageID)

		t.Approvers(currentMessageID).Consume(func(approver *Approver) {
			approverID := approver.ApproverMessageID()
			if _, messageProcessed := processedMessages[approverID]; !messageProcessed {
				cleanupStack.PushBack(approverID)
				processedMessages[approverID] = types.Void
			}
		})
	}
}

// SolidifierWorkerPoolStatus returns the name and the load of the workerpool.
func (t *Tangle) SolidifierWorkerPoolStatus() (name string, load int) {
	return "Solidifier", t.solidifierWorkerPool.RunningWorkers()
}
