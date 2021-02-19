package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// attachmentsHandler gets the value attachments.
func attachmentsHandler(c echo.Context) error {
	txnID, err := ledgerstate.TransactionIDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, AttachmentsResponse{Error: err.Error()})
	}

	var valueObjs []ValueObject

	// get txn by txn id
	tx := messagelayer.Tangle().LedgerState.Transaction(txnID)
	defer tx.Release()
	if !tx.Exists() {
		return c.JSON(http.StatusNotFound, AttachmentsResponse{Error: "Transaction not found"})
	}
	txn := ParseTransaction(tx.Unwrap())

	// get attachments by txn id
	for _, attachmentObj := range messagelayer.Tangle().Storage.Attachments(txnID) {
		defer attachmentObj.Release()
		if !attachmentObj.Exists() {
			continue
		}
		attachment := attachmentObj.Unwrap()

		// get payload by payload id
		payloadObj := messagelayer.Tangle().Storage.Message(attachment.MessageID())
		defer payloadObj.Release()
		if !payloadObj.Exists() {
			continue
		}
		payload := payloadObj.Unwrap()
		var parents []string
		for _, parent := range payload.Parents() {
			parents = append(parents, parent.String())
		}
		// append value object
		valueObjs = append(valueObjs, ValueObject{
			ID:          payload.ID().String(),
			Parents:     parents,
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
	Parents     []string    `json:"parents"`
	Transaction Transaction `json:"transaction"`
}
