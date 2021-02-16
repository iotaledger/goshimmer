package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// TipsHandler gets the value object info from the tips.
func TipsHandler(c echo.Context) error {
	tipManager := messagelayer.Tangle().TipManager
	var result []Object
	for _, msgID := range tipManager.Tips(tipManager.TipCount()) {
		var obj Object

		cachedMessage := messagelayer.Tangle().Storage.Message(msgID)
		message := cachedMessage.Unwrap()
		cachedMessage.Release()
		if message.Payload().Type() == ledgerstate.TransactionType {
			tx := message.Payload().(*ledgerstate.Transaction)
			inclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(tx.ID())
			if err != nil {
				return c.JSON(http.StatusBadRequest, TipsResponse{Error: err.Error()})
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
			result = append(result, obj)
		}
	}
	return c.JSON(http.StatusOK, TipsResponse{ValueObjects: result})
}

// TipsResponse is the HTTP response from retrieving value objects.
type TipsResponse struct {
	ValueObjects []Object `json:"value_objects,omitempty"`
	Error        string   `json:"error,omitempty"`
}
