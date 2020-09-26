package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// attachmentsHandler gets the value attachments.
func attachmentsHandler(c echo.Context) error {
	txnID, err := transaction.IDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, AttachmentsResponse{Error: err.Error()})
	}

	var valueObjs []ValueObject

	// get txn by txn id
	txnObj := valuetransfers.Tangle().Transaction(txnID)
	defer txnObj.Release()
	if !txnObj.Exists() {
		return c.JSON(http.StatusNotFound, AttachmentsResponse{Error: "Transaction not found"})
	}
	txn := ParseTransaction(txnObj.Unwrap())

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
			Parent1ID:   payload.Parent1ID().String(),
			Parent2ID:   payload.Parent2ID().String(),
			Transaction: txn,
		})
	}

	return c.JSON(http.StatusOK, AttachmentsResponse{Attachments: valueObjs})
}

// AttachmentsResponse is the HTTP response from retrieving value objects.
type AttachmentsResponse struct {
	Attachments []ValueObject `json:"attachments,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// ValueObject holds the information of a value object.
type ValueObject struct {
	ID          string      `json:"id"`
	Parent1ID   string      `json:"parent1_id"`
	Parent2ID   string      `json:"parent2_id"`
	Transaction Transaction `json:"transaction"`
}
