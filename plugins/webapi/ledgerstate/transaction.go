package ledgerstate

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

// region Errors //////////////////////////////////////////////////////////////////////////////////////////////////////
var (
	// ErrTransactionNotFound is returned if the transaction is not found.
	ErrTransactionNotFound = xerrors.New("transaction not found")
	// ErrTransactionMetadataNotFound is returned if the transaction metadata is not found.
	ErrTransactionMetadataNotFound = xerrors.New("transaction metadata not found")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetTransactionByIDEndpoint is the handler for /ledgerstate/transactions/:transactionID endpoint.
func GetTransactionByIDEndpoint(c echo.Context) (err error) {
	txID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}
	cachedTxnObj := messagelayer.Tangle().LedgerState.Transaction(txID)
	defer cachedTxnObj.Release()
	if !cachedTxnObj.Exists() {
		return c.JSON(http.StatusNotFound, NewErrorResponse(ErrTransactionNotFound))
	}
	txn := value.ParseTransaction(cachedTxnObj.Unwrap())
	return c.JSON(http.StatusOK, txn)
}

// GetTransactionMetadataEndpoint is the handler for ledgerstate/transactions/:transactionID/metadata
func GetTransactionMetadataEndpoint(c echo.Context) (err error) {
	txID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}
	cachedTxnMeta := messagelayer.Tangle().LedgerState.TransactionMetadata(txID)
	defer cachedTxnMeta.Release()
	if !cachedTxnMeta.Exists() {
		return c.JSON(http.StatusNotFound, NewErrorResponse(ErrTransactionMetadataNotFound))
	}
	txMetadata := cachedTxnMeta.Unwrap()
	return c.JSON(http.StatusOK, value.TransactionMetadata{
		BranchID:           txMetadata.BranchID().Base58(),
		Solid:              txMetadata.Solid(),
		SolidificationTime: txMetadata.SolidificationTime().Unix(),
		Finalized:          txMetadata.Finalized(),
		LazyBooked:         txMetadata.LazyBooked(),
	})
}

// GetTransactionAttachmentsEndpoint is the handler for ledgerstate/transactions/:transactionID/attachments endpoint.
func GetTransactionAttachmentsEndpoint(c echo.Context) (err error) {
	txID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}
	cachedAttachments := messagelayer.Tangle().Storage.Attachments(txID)
	var messageIDs tangle.MessageIDs
	cachedAttachments.Consume(func(attachment *tangle.Attachment) {
		messageIDs = append(messageIDs, attachment.MessageID())
	})
	return c.JSON(http.StatusOK, NewAttachments(txID, messageIDs))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Attachments ////////////////////////////////////////////////////////////////////////////////////////////////

// Attachments is the JSON model for the attachments of a transaction.
type Attachments struct {
	TransactionID string   `json:"transactionID"`
	MessageIDs    []string `json:"messageIDs"`
}

// NewAttachments creates a new `Attachments`.
func NewAttachments(transactionID ledgerstate.TransactionID, messageIDs tangle.MessageIDs) Attachments {
	var messageIDsBase58 []string
	for _, messageID := range messageIDs {
		messageIDsBase58 = append(messageIDsBase58, messageID.String())
	}
	return Attachments{
		TransactionID: transactionID.Base58(),
		MessageIDs:    messageIDsBase58,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
