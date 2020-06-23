package sendtransaction

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/labstack/echo"
)

// Handler sends a transaction.
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// prepare transaction
	tx, _, err := transaction.FromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// Prepare value payload and send the message to tangle
	payload := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	_, err = issuer.IssuePayload(payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{TransactionID: tx.ID().String()})
}

// Request holds the transaction object(bytes) to send.
type Request struct {
	TransactionBytes []byte `json:"txn_bytes"`
}

// Response is the HTTP response from sending transaction.
type Response struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}
