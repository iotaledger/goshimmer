package value

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

const maxBookedAwaitTime = 5 * time.Second

var (
	sendTxMu sync.Mutex
	// ErrNotAllowedToPledgeManaToNode defines an unsupported node to pledge mana to.
	ErrNotAllowedToPledgeManaToNode = errors.New("not allowed to pledge mana to node")
)

// sendTransactionHandler sends a transaction.
func sendTransactionHandler(c echo.Context) error {
	sendTxMu.Lock()
	defer sendTxMu.Unlock()

	var request value.SendTransactionRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: err.Error()})
	}

	// parse tx
	tx, _, err := ledgerstate.TransactionFromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: err.Error()})
	}

	// validate allowed mana pledge nodes.
	allowedAccessMana := messagelayer.GetAllowedPledgeNodes(mana.AccessMana)
	if allowedAccessMana.IsFilterEnabled {
		if !allowedAccessMana.Allowed.Has(tx.Essence().AccessPledgeID()) {
			return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{
				Error: fmt.Errorf("not allowed to pledge access mana to %s: %w", tx.Essence().AccessPledgeID().String(), ErrNotAllowedToPledgeManaToNode).Error(),
			})
		}
	}
	allowedConsensusMana := messagelayer.GetAllowedPledgeNodes(mana.ConsensusMana)
	if allowedConsensusMana.IsFilterEnabled {
		if !allowedConsensusMana.Allowed.Has(tx.Essence().ConsensusPledgeID()) {
			return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{
				Error: fmt.Errorf("not allowed to pledge consensus mana to %s: %w", tx.Essence().ConsensusPledgeID().String(), ErrNotAllowedToPledgeManaToNode).Error(),
			})
		}
	}

	// check transaction validity
	if transactionErr := messagelayer.Tangle().LedgerState.CheckTransaction(tx); transactionErr != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: transactionErr.Error()})
	}

	// check if transaction is too old
	if tx.Essence().Timestamp().Before(clock.SyncedTime().Add(-tangle.MaxReattachmentTimeMin)) {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: fmt.Sprintf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangle.MaxReattachmentTimeMin)})
	}

	// if transaction is in the future we wait until the time arrives
	if tx.Essence().Timestamp().After(clock.SyncedTime()) {
		time.Sleep(tx.Essence().Timestamp().Sub(clock.SyncedTime()) + 1*time.Nanosecond)
	}

	issueTransaction := func() (*tangle.Message, error) {
		return messagelayer.Tangle().IssuePayload(tx)
	}

	if _, err := messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime); err != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, value.SendTransactionResponse{TransactionID: tx.ID().Base58()})
}
