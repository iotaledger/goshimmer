package gettransactionbyid

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler gets the transaction by id.
func Handler(c echo.Context) error {
	txnID, err := transaction.IDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// get txn by txn id
	txnObj := valuetransfers.Tangle().Transaction(txnID)
	defer txnObj.Release()
	if !txnObj.Exists() {
		return c.JSON(http.StatusNotFound, Response{Error: "Transaction not found"})
	}
	txn := utils.ParseTransaction(txnObj.Unwrap())

	// get txn metadata
	txnMetadataObj := valuetransfers.Tangle().TransactionMetadata(txnID)
	defer txnMetadataObj.Release()
	if !txnMetadataObj.Exists() {
		return c.JSON(http.StatusNotFound, Response{Error: "Transaction Metadata not found"})
	}
	txnMetadata := txnMetadataObj.Unwrap()

	return c.JSON(http.StatusOK, Response{
		Transaction: txn,
		InclusionState: utils.InclusionState{
			Solid:     txnMetadata.Solid(),
			Confirmed: txnMetadata.Confirmed(),
			Rejected:  txnMetadata.Rejected(),
			Liked:     txnMetadata.Liked(),
		},
	})
}

// Response is the HTTP response from retreiving transaction.
type Response struct {
	Transaction    utils.Transaction    `json:"transaction,omitempty"`
	InclusionState utils.InclusionState `json:"inclusion_state,omitempty"`
	Error          string               `json:"error,omitempty"`
}
