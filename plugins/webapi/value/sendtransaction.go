package value

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

var sendTxMu sync.Mutex

// sendTransactionHandler sends a transaction.
func sendTransactionHandler(c echo.Context) error {
	sendTxMu.Lock()
	defer sendTxMu.Unlock()

	var request SendTransactionRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionResponse{Error: err.Error()})
	}

	// parse tx
	tx, _, err := ledgerstate.TransactionFromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionResponse{Error: err.Error()})
	}

	issueTransaction := func() (*tangle.Message, error) {
		msg, e := issuer.IssuePayload(tx, messagelayer.Tangle())
		if e != nil {
			return nil, c.JSON(http.StatusBadRequest, SendTransactionResponse{Error: e.Error()})
		}
		return msg, nil
	}

	_, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, SendTransactionResponse{TransactionID: tx.ID().Base58()})
}

// SendTransactionRequest holds the transaction object(bytes) to send.
type SendTransactionRequest struct {
	TransactionBytes []byte `json:"txn_bytes"`
}

// SendTransactionResponse is the HTTP response from sending transaction.
type SendTransactionResponse struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}
