package value

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

// getTransactionByIDHandler gets the transaction by id.
func getTransactionByIDHandler(c echo.Context) error {
	txID, err := ledgerstate.TransactionIDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, value.GetTransactionByIDResponse{Error: err.Error()})
	}

	// get txn by txn id
	cachedTxnMetaObj := messagelayer.Tangle().LedgerState.TransactionMetadata(txID)
	defer cachedTxnMetaObj.Release()
	if !cachedTxnMetaObj.Exists() {
		return c.JSON(http.StatusNotFound, value.GetTransactionByIDResponse{Error: "Transaction not found"})
	}
	cachedTxnObj := messagelayer.Tangle().LedgerState.Transaction(txID)
	defer cachedTxnObj.Release()
	if !cachedTxnObj.Exists() {
		return c.JSON(http.StatusNotFound, value.GetTransactionByIDResponse{Error: "Transaction not found"})
	}
	txn := value.ParseTransaction(cachedTxnObj.Unwrap())

	txMetadata := cachedTxnMetaObj.Unwrap()
	txInclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(txID)
	if err != nil {
		return c.JSON(http.StatusOK, value.GetTransactionByIDResponse{Error: err.Error()})
	}
	cachedBranch := messagelayer.Tangle().LedgerState.BranchDAG.Branch(txMetadata.BranchID())
	defer cachedBranch.Release()
	if !cachedTxnObj.Exists() {
		return c.JSON(http.StatusNotFound, value.GetTransactionByIDResponse{Error: "Branch not found"})
	}
	branch := cachedBranch.Unwrap()

	return c.JSON(http.StatusOK, value.GetTransactionByIDResponse{
		TransactionMetadata: value.TransactionMetadata{
			BranchID:           branch.ID().String(),
			Solid:              txMetadata.Solid(),
			SolidificationTime: txMetadata.SolidificationTime().Unix(),
			Finalized:          txMetadata.Finalized(),
			LazyBooked:         txMetadata.LazyBooked(),
		},
		Transaction: txn,
		InclusionState: value.InclusionState{
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
