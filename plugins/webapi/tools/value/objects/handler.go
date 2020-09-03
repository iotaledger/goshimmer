package objects

import (
	"container/list"
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/labstack/echo"
)

// Handler returns the list of value objects.
func Handler(c echo.Context) error {
	result := []ValueObject{}

	for _, valueID := range Approvers(payload.GenesisID) {
		var obj ValueObject

		valuetransfers.Tangle().Payload(valueID).Consume(func(p *payload.Payload) {
			obj.Parent1 = p.TrunkID().String()
			obj.Parent2 = p.BranchID().String()
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
	return c.JSON(http.StatusOK, Response{ValueObjects: result})
}

// Response is the HTTP response from retrieving value objects.
type Response struct {
	ValueObjects []ValueObject `json:"value_objects,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// ValueObject holds the info of a ValueObject
type ValueObject struct {
	Parent1       string `json:"parent_1"`
	Parent2       string `json:"parent_2"`
	ID            string `json:"id"`
	Tip           bool   `json:"tip"`
	Solid         bool   `json:"solid"`
	Liked         bool   `json:"liked"`
	Confirmed     bool   `json:"confirmed"`
	Rejected      bool   `json:"rejected"`
	BranchID      string `json:"branch_id"`
	TransactionID string `json:"transaction_id"`
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
