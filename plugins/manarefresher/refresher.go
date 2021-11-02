package manarefresher

import (
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// maxBookedAwaitTime is the time the refresher waits for an issued tx to become booked.
const maxBookedAwaitTime = 5 * time.Second

// Refresher is a component that takes care of refreshing the mana delegated to the node.
type Refresher struct {
	wallet   *wallet
	receiver *DelegationReceiver
}

// NewRefresher creates a new Refresher object.
func NewRefresher(wallet *wallet, receiver *DelegationReceiver) *Refresher {
	return &Refresher{
		wallet:   wallet,
		receiver: receiver,
	}
}

// Refresh scans the tangle for delegated outputs, and refreshes the node's mana by moving those.
func (r *Refresher) Refresh() (err error) {
	delegationOutputs := r.receiver.Scan()
	if len(delegationOutputs) == 0 {
		return
	}
	numberOfChunks := len(delegationOutputs)/ledgerstate.MaxInputCount + 1

	for i := 0; i < numberOfChunks; i++ {
		// which chunks to consume?
		var consumedChunk []*ledgerstate.AliasOutput
		if len(delegationOutputs) > ledgerstate.MaxInputCount {
			consumedChunk = delegationOutputs[:ledgerstate.MaxInputCount]
			delegationOutputs = delegationOutputs[ledgerstate.MaxInputCount:]
		} else {
			consumedChunk = delegationOutputs
		}

		var tx *ledgerstate.Transaction
		tx, err = r.prepareRefreshingTransaction(consumedChunk)
		if err != nil {
			return
		}
		err = r.sendTransaction(tx)
		if err != nil {
			return
		}
	}
	return nil
}

// prepareRefreshingTransaction prepares a transaction moving delegated outputs with state transition only and pledging mana
// to the node itself.
func (r *Refresher) prepareRefreshingTransaction(toBeRefreshed []*ledgerstate.AliasOutput) (tx *ledgerstate.Transaction, err error) {
	// prepare inputs
	inputs := make(ledgerstate.Inputs, len(toBeRefreshed))
	for k, output := range toBeRefreshed {
		inputs[k] = output.Input()
	}
	// prepare outputs
	outputs := make(ledgerstate.Outputs, len(toBeRefreshed))
	for k, alias := range toBeRefreshed {
		outputs[k] = alias.NewAliasOutputNext(false)
	}
	// prepare essence
	essence := ledgerstate.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		deps.Local.ID(), // pledge both manas to self
		deps.Local.ID(),
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)
	tx = ledgerstate.NewTransaction(essence, r.wallet.unlockBlocks(essence))

	// check transaction validity
	if transactionErr := deps.Tangle.LedgerState.CheckTransaction(tx); transactionErr != nil {
		return nil, transactionErr
	}

	// check if transaction is too old
	if tx.Essence().Timestamp().Before(clock.SyncedTime().Add(-tangle.MaxReattachmentTimeMin)) {
		return nil, errors.Errorf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangle.MaxReattachmentTimeMin)
	}

	return tx, nil
}

func (r *Refresher) sendTransaction(tx *ledgerstate.Transaction) (err error) {
	issueTransaction := func() (*tangle.Message, error) {
		return deps.Tangle.IssuePayload(tx)
	}
	if _, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime); err != nil {
		return err
	}
	return nil
}
