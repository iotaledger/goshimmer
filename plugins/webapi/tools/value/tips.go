package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/labstack/echo"
)

// TipsHandler gets the value object info from the tips.
func TipsHandler(c echo.Context) error {
	result := make([]Object, valuetransfers.TipManager().Size())
	for i, valueID := range valuetransfers.TipManager().AllTips() {
		var obj Object
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
	return c.JSON(http.StatusOK, TipsResponse{ValueObjects: result})
}

// TipsResponse is the HTTP response from retrieving value objects.
type TipsResponse struct {
	ValueObjects []Object `json:"value_objects,omitempty"`
	Error        string   `json:"error,omitempty"`
}
