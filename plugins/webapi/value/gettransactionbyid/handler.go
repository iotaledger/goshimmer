package gettransactionbyid

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/labstack/echo"
)

// Handler gets the transaction by id.
func Handler(c echo.Context) error {
	txnID, err := transaction.IDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// get txn by txn id
	cachedTxnMetaObj := valuetransfers.Tangle().TransactionMetadata(txnID)
	defer cachedTxnMetaObj.Release()
	if !cachedTxnMetaObj.Exists() {
		return c.JSON(http.StatusNotFound, Response{Error: "Transaction not found"})
	}
	cachedTxnObj := valuetransfers.Tangle().Transaction(txnID)
	defer cachedTxnObj.Release()
	if !cachedTxnObj.Exists() {
		return c.JSON(http.StatusNotFound, Response{Error: "Transaction not found"})
	}
	txn := utils.ParseTransaction(cachedTxnObj.Unwrap())

	txnMeta := cachedTxnMetaObj.Unwrap()
	txnMeta.Preferred()
	return c.JSON(http.StatusOK, Response{
		Transaction: txn,
		InclusionState: utils.InclusionState{
			Confirmed:   txnMeta.Confirmed(),
			Conflicting: txnMeta.Conflicting(),
			Liked:       txnMeta.Liked(),
			Solid:       txnMeta.Solid(),
			Rejected:    txnMeta.Rejected(),
			Finalized:   txnMeta.Finalized(),
			Preferred:   txnMeta.Preferred(),
		},
	})
}

// Response is the HTTP response from retrieving transaction.
type Response struct {
	Transaction    utils.Transaction    `json:"transaction,omitempty"`
	InclusionState utils.InclusionState `json:"inclusion_state,omitempty"`
	Error          string               `json:"error,omitempty"`
}
