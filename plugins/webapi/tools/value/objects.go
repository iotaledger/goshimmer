package value

import (
	"container/list"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// ObjectsHandler returns the list of value objects.
func ObjectsHandler(c echo.Context) error {
	result := []Object{}

	for _, msgID := range Approvers(tangle.EmptyMessageID) {
		var obj Object
		cachedMessage := messagelayer.Tangle().Storage.Message(msgID)
		message := cachedMessage.Unwrap()
		cachedMessage.Release()
		tx := message.Payload().(*ledgerstate.Transaction)
		inclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(tx.ID())
		if err != nil {
			return c.JSON(http.StatusBadRequest, ObjectsResponse{Error: err.Error()})
		}
		messagelayer.Tangle().LedgerState.TransactionMetadata(tx.ID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			obj.Solid = transactionMetadata.Solid()
			obj.Finalized = transactionMetadata.Finalized()
		})
		obj.ID = tx.ID().String()
		obj.InclusionState = inclusionState.String()
		obj.BranchID = messagelayer.Tangle().LedgerState.BranchID(tx.ID()).String()
		for _, parent := range message.Parents() {
			obj.Parents = append(obj.Parents, parent.String())
		}

		obj.Tip = !messagelayer.Tangle().Storage.Approvers(message.ID()).Consume(func(approver *tangle.Approver) {})

		result = append(result, obj)
	}
	return c.JSON(http.StatusOK, ObjectsResponse{ValueObjects: result})
}

// ObjectsResponse is the HTTP response from retrieving value objects.
type ObjectsResponse struct {
	ValueObjects []Object `json:"value_objects,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Approvers returns the list of approvers up to the tips.
func Approvers(msgID tangle.MessageID) tangle.MessageIDs {
	visited := make(map[tangle.MessageID]bool)
	queue := list.New()
	queue.PushBack(msgID)
	visited[msgID] = true

	for queue.Len() > 0 {
		qnode := queue.Front()
		// iterate through all of its approvers
		// mark the visited ids; enqueue the non-visted
		for _, id := range getApprovers(qnode.Value.(tangle.MessageID)) {
			if _, ok := visited[id]; !ok {
				visited[id] = true
				queue.PushBack(id)
			}
		}
		queue.Remove(qnode)
	}

	msgIDs := make(tangle.MessageIDs, 0)
	// collect all the ids into slice
	for msgID := range visited {
		messagelayer.Tangle().Storage.Message(msgID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == ledgerstate.TransactionType {
				msgIDs = append(msgIDs, msgID)
			}
		})
	}

	return msgIDs
}

func getApprovers(msgID tangle.MessageID) (messageIDs tangle.MessageIDs) {
	messagelayer.Tangle().Storage.Approvers(msgID).Consume(func(approver *tangle.Approver) {
		messageIDs = append(messageIDs, approver.ApproverMessageID())
	})
	return
}
