package testing

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/waspconn"
	"github.com/iotaledger/goshimmer/packages/waspconn/apilib"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/labstack/echo"
)

func addEndpoints(vtangle waspconn.Ledger) {
	t := &testingHandler{vtangle}

	// TODO: replace real webapi endpoints, so that client does not need to know if
	// utxodb is being used

	// GET /value/unspentOutputs
	webapi.Server().GET("/utxodb/outputs/:address", t.handleGetAddressOutputs)
	// GET /value/transactionByID
	webapi.Server().GET("/utxodb/inclusionstate/:txid", t.handleInclusionState)
	// POST /value/sendTransaction
	webapi.Server().POST("/utxodb/tx", t.handlePostTransaction)

	// POST /faucet
	webapi.Server().GET("/utxodb/requestfunds/:address", t.handleRequestFunds)

	// TODO: this endpoint is only needed for wasp-cluster, because there is no way
	// to programatically send a SIGINT to the process in Windows.
	// There is a proposed solution here: https://github.com/golang/go/issues/28498
	webapi.Server().GET("/adm/shutdown", handleShutdown)

	log.Info("addded UTXODB endpoints")
}

type testingHandler struct {
	vtangle waspconn.Ledger
}

func (t *testingHandler) handleGetAddressOutputs(c echo.Context) error {
	addr, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, &value.UnspentOutputsResponse{Error: err.Error()})
	}

	var outs []value.OutputID
	t.vtangle.GetUnspentOutputs(addr, func(output ledgerstate.Output) {
		var b []value.Balance
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			b = append(b, value.Balance{
				Value: int64(balance),
				Color: color.String(),
			})
			return true
		})

		var timestamp time.Time
		t.vtangle.GetConfirmedTransaction(output.ID().TransactionID(), func(tx *ledgerstate.Transaction) {
			timestamp = tx.Essence().Timestamp()
		})

		outs = append(outs, value.OutputID{
			ID:             output.ID().Base58(),
			Balances:       b,
			InclusionState: value.InclusionState{Confirmed: true},
			Metadata:       value.Metadata{Timestamp: timestamp},
		})
	})
	return c.JSON(http.StatusOK, &value.UnspentOutputsResponse{
		UnspentOutputs: []value.UnspentOutput{
			value.UnspentOutput{
				Address:   addr.Base58(),
				OutputIDs: nil,
			},
		},
	})
}

func (t *testingHandler) handlePostTransaction(c echo.Context) error {
	var req apilib.PostTransactionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &apilib.PostTransactionResponse{Err: err.Error()})
	}

	tx, _, err := ledgerstate.TransactionFromBytes(req.Tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &apilib.PostTransactionResponse{Err: err.Error()})
	}

	err = t.vtangle.PostTransaction(tx)
	if err != nil {
		return c.JSON(http.StatusConflict, &apilib.PostTransactionResponse{Err: err.Error()})
	}

	return c.JSON(http.StatusOK, &apilib.PostTransactionResponse{})
}

func handleShutdown(c echo.Context) error {
	gracefulshutdown.ShutdownWithError(fmt.Errorf("Shutdown requested from WebAPI."))
	return nil
}

func (t *testingHandler) handleInclusionState(c echo.Context) error {
	txid, err := ledgerstate.TransactionIDFromBase58(c.Param("txid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, &apilib.InclusionStateResponse{Err: err.Error()})
	}
	state, err := t.vtangle.GetTxInclusionState(txid)
	if err != nil {
		return c.JSON(http.StatusNotFound, &apilib.InclusionStateResponse{Err: err.Error()})
	}

	return c.JSON(http.StatusOK, &apilib.InclusionStateResponse{State: state})
}

func (t *testingHandler) handleRequestFunds(c echo.Context) error {
	addr, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, &apilib.RequestFundsResponse{Err: err.Error()})
	}
	err = t.vtangle.RequestFunds(addr)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &apilib.RequestFundsResponse{Err: err.Error()})
	}
	return c.JSON(http.StatusOK, &apilib.RequestFundsResponse{})
}
