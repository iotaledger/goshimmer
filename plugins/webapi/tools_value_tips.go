package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/labstack/echo"
)

func init() {
	Server().GET("tools/value/tips", tipsHandler)
}

// tipsHandler gets the value object info from the tips.
func tipsHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[ToolsRoot]; exists {
		return c.JSON(http.StatusForbidden, TipsResponse{Error: "Forbidden endpoint"})
	}

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
	return c.JSON(http.StatusOK, TipsResponse{ValueObjects: result})
}

// TipsResponse is the HTTP response from retrieving value objects.
type TipsResponse struct {
	ValueObjects []ValueObject `json:"value_objects,omitempty"`
	Error        string        `json:"error,omitempty"`
}
