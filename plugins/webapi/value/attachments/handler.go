package attachments

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler gets the value attachments.
func Handler(c echo.Context) error {
	txnID, err := transaction.IDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var valueObjs []ValueObject

	// get txn by txn id
	txnObj := valuetransfers.Tangle().Transaction(txnID)
	defer txnObj.Release()
	if !txnObj.Exists() {
		return c.JSON(http.StatusNotFound, Response{Error: "Transaction not found"})
	}
	txn := utils.ParseTransaction(txnObj.Unwrap())

	// get attachments by txn id
	for _, attachmentObj := range valuetransfers.Tangle().Attachments(txnID) {
		defer attachmentObj.Release()
		if !attachmentObj.Exists() {
			continue
		}
		attachment := attachmentObj.Unwrap()

		// get payload by payload id
		payloadObj := valuetransfers.Tangle().Payload(attachment.PayloadID())
		defer payloadObj.Release()
		if !payloadObj.Exists() {
			continue
		}
		payload := payloadObj.Unwrap()

		// append value object
		valueObjs = append(valueObjs, ValueObject{
			ID:          payload.ID().String(),
			ParentID0:   payload.TrunkID().String(),
			ParentID1:   payload.BranchID().String(),
			Transaction: txn,
		})
	}

	return c.JSON(http.StatusOK, Response{Attachments: valueObjs})
}

// Response is the HTTP response from retrieving value objects.
type Response struct {
	Attachments []ValueObject `json:"attachments,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// ValueObject holds the information of a value object.
type ValueObject struct {
	ID          string            `json:"id"`
	ParentID0   string            `json:"parent0_id"`
	ParentID1   string            `json:"parent1_id"`
	Transaction utils.Transaction `json:"transaction"`
}
