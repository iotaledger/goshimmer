package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/labstack/echo"
)

func init() {
	Server().GET("value/attachments", attachmentsHandler)
}

// attachmentsHandler gets the value attachments.
func attachmentsHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[ValueRoot]; exists {
		return c.JSON(http.StatusForbidden, AttachmentsResponse{Error: "Forbidden endpoint"})
	}

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
			Parent1:     payload.Parent1ID().String(),
			Parent2:     payload.Parent2ID().String(),
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
