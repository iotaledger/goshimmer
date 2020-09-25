package value

import (
	"container/list"
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/labstack/echo"
)

// ObjectsHandler returns the list of value objects.
func ObjectsHandler(c echo.Context) error {
	result := []Object{}

	for _, valueID := range Approvers(payload.GenesisID) {
		var obj Object

		valuetransfers.Tangle().Payload(valueID).Consume(func(p *payload.Payload) {
			obj.Parent1 = p.Parent1ID().String()
			obj.Parent2 = p.Parent2ID().String()
			obj.TransactionID = p.Transaction().ID().String()
		})

		valuetransfers.Tangle().PayloadMetadata(valueID).Consume(func(payloadMetadata *tangle.PayloadMetadata) {
			obj.ID = payloadMetadata.PayloadID().String()
			obj.Solid = payloadMetadata.IsSolid()
			obj.Liked = payloadMetadata.Liked()
			obj.Confirmed = payloadMetadata.Confirmed()
			obj.Rejected = payloadMetadata.Rejected()
			obj.BranchID = payloadMetadata.BranchID().String()
		})

		obj.Tip = !valuetransfers.Tangle().Approvers(valueID).Consume(func(approver *tangle.PayloadApprover) {})

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
func Approvers(id payload.ID) []payload.ID {
	visited := make(map[payload.ID]bool)
	queue := list.New()
	queue.PushBack(id)
	visited[id] = true

	for queue.Len() > 0 {
		qnode := queue.Front()
		// iterate through all of its approvers
		// mark the visited ids; enqueue the non-visted
		for _, id := range getApprovers(qnode.Value.(payload.ID)) {
			if _, ok := visited[id]; !ok {
				visited[id] = true
				queue.PushBack(id)
			}
		}
		queue.Remove(qnode)
	}

	ids := make([]payload.ID, 0)
	// collect all the ids into slice
	for id := range visited {
		ids = append(ids, id)
	}

	return ids
}

func getApprovers(id payload.ID) []payload.ID {
	result := []payload.ID{}
	valuetransfers.Tangle().Approvers(id).Consume(func(approver *tangle.PayloadApprover) {
		result = append(result, approver.ApprovingPayloadID())
	})
	return result
}
