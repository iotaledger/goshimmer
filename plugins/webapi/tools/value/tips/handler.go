package tips

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/labstack/echo"
)

// Handler gets the value object info from the tips.
func Handler(c echo.Context) error {
	result := make([]ValueObject, valuetransfers.TipManager().Size())
	for i, valueID := range valuetransfers.TipManager().AllTips() {
		var obj ValueObject
		valuetransfers.Tangle().PayloadMetadata(valueID).Consume(func(payloadMetadata *tangle.PayloadMetadata) {
			obj.ID = payloadMetadata.PayloadID().String()
			obj.Solid = payloadMetadata.IsSolid()
			obj.Liked = payloadMetadata.Liked()
			obj.Confirmed = payloadMetadata.Confirmed()
			obj.Rejected = payloadMetadata.Rejected()
			obj.BranchID = payloadMetadata.BranchID().String()
		})

		valuetransfers.Tangle().Payload(valueID).Consume(func(p *payload.Payload) {
			obj.TransactionID = p.Transaction().ID().String()
		})

		result[i] = obj
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
	ID            string `json:"id"`
	Solid         bool   `json:"solid"`
	Liked         bool   `json:"liked"`
	Confirmed     bool   `json:"confirmed"`
	Rejected      bool   `json:"rejected"`
	BranchID      string `json:"branch_id"`
	TransactionID string `json:"transaction_id"`
}
