package sendtransaction

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/labstack/echo"
)

var (
	sendTxMu           sync.Mutex
	maxBookedAwaitTime = 5 * time.Second

	// ErrNotAllowedToPledgeManaToNode defines an unsupported node to pledge mana to.
	ErrNotAllowedToPledgeManaToNode = errors.New("not allowed to pledge mana to node")
)

// Handler sends a transaction.
func Handler(c echo.Context) error {
	sendTxMu.Lock()
	defer sendTxMu.Unlock()

	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// prepare transaction
	tx, _, err := transaction.FromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	allowedAccessMana := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	if allowedAccessMana.IsFilterEnabled {
		if !allowedAccessMana.Allowed.Has(tx.AccessManaNodeID()) {
			return c.JSON(http.StatusBadRequest, Response{
				Error: fmt.Errorf("not allowed to pledge access mana to %s: %w", tx.AccessManaNodeID().String(), ErrNotAllowedToPledgeManaToNode).Error(),
			})
		}
	}
	allowedConsensusMana := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)
	if allowedConsensusMana.IsFilterEnabled {
		if !allowedConsensusMana.Allowed.Has(tx.ConsensusManaNodeID()) {
			return c.JSON(http.StatusBadRequest, Response{
				Error: fmt.Errorf("not allowed to pledge consensus mana to %s: %w", tx.ConsensusManaNodeID().String(), ErrNotAllowedToPledgeManaToNode).Error(),
			})
		}
	}

	err = valuetransfers.Tangle().ValidateTransactionToAttach(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// Prepare value payload and send the message to tangle
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	_, err = issuer.IssuePayload(payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	if err := valuetransfers.AwaitTransactionToBeBooked(tx.ID(), maxBookedAwaitTime); err != nil {
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
