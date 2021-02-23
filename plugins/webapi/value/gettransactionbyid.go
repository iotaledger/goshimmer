package value

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// getTransactionByIDHandler gets the transaction by id.
func getTransactionByIDHandler(c echo.Context) error {
	txID, err := ledgerstate.TransactionIDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetTransactionByIDResponse{Error: err.Error()})
	}

	// get txn by txn id
	cachedTxnMetaObj := messagelayer.Tangle().LedgerState.TransactionMetadata(txID)
	defer cachedTxnMetaObj.Release()
	if !cachedTxnMetaObj.Exists() {
		return c.JSON(http.StatusNotFound, GetTransactionByIDResponse{Error: "Transaction not found"})
	}
	cachedTxnObj := messagelayer.Tangle().LedgerState.Transaction(txID)
	defer cachedTxnObj.Release()
	if !cachedTxnObj.Exists() {
		return c.JSON(http.StatusNotFound, GetTransactionByIDResponse{Error: "Transaction not found"})
	}
	txn := ParseTransaction(cachedTxnObj.Unwrap())

	txMetadata := cachedTxnMetaObj.Unwrap()
	txInclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(txID)
	if err != nil {
		return c.JSON(http.StatusOK, GetTransactionByIDResponse{Error: err.Error()})
	}
	cachedBranch := messagelayer.Tangle().LedgerState.Branch(txMetadata.BranchID())
	defer cachedBranch.Release()
	if !cachedTxnObj.Exists() {
		return c.JSON(http.StatusNotFound, GetTransactionByIDResponse{Error: "Branch not found"})
	}
	branch := cachedBranch.Unwrap()

	return c.JSON(http.StatusOK, GetTransactionByIDResponse{
		Transaction: txn,
		InclusionState: InclusionState{
			Confirmed:   txInclusionState == ledgerstate.Confirmed,
			Conflicting: messagelayer.Tangle().LedgerState.TransactionConflicting(txID),
			Liked:       branch.Liked(),
			Solid:       txMetadata.Solid(),
			Rejected:    txInclusionState == ledgerstate.Rejected,
			Finalized:   txMetadata.Finalized(),
			Preferred:   false,
		},
	})
}

// GetTransactionByIDResponse is the HTTP response from retrieving transaction.
type GetTransactionByIDResponse struct {
	Transaction    Transaction    `json:"transaction,omitempty"`
	InclusionState InclusionState `json:"inclusion_state,omitempty"`
	Error          string         `json:"error,omitempty"`
}
