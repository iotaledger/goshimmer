package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"

	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// attachmentsHandler gets the value attachments.
func attachmentsHandler(c echo.Context) error {
	txnID, err := ledgerstate.TransactionIDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, value.AttachmentsResponse{Error: err.Error()})
	}

	var valueObjs []value.ValueObject

	// get txn by txn id
	tx := messagelayer.Tangle().LedgerState.Transaction(txnID)
	defer tx.Release()
	if !tx.Exists() {
		return c.JSON(http.StatusNotFound, value.AttachmentsResponse{Error: "Transaction not found"})
	}
	txn := value.ParseTransaction(tx.Unwrap())

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
			parents = append(parents, parent.Base58())
		}
		// append value object
		valueObjs = append(valueObjs, value.ValueObject{
			ID:          payload.ID().Base58(),
			Parents:     parents,
			Transaction: txn,
		})
	}

	return c.JSON(http.StatusOK, value.AttachmentsResponse{Attachments: valueObjs})
}
