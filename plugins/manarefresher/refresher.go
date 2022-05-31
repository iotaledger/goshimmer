package manarefresher

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
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
	numberOfChunks := len(delegationOutputs)/devnetvm.MaxInputCount + 1

	for i := 0; i < numberOfChunks; i++ {
		// which chunks to consume?
		var consumedChunk []*devnetvm.AliasOutput
		if len(delegationOutputs) > devnetvm.MaxInputCount {
			consumedChunk = delegationOutputs[:devnetvm.MaxInputCount]
			delegationOutputs = delegationOutputs[devnetvm.MaxInputCount:]
		} else {
			consumedChunk = delegationOutputs
		}

		var tx *devnetvm.Transaction
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
func (r *Refresher) prepareRefreshingTransaction(toBeRefreshed []*devnetvm.AliasOutput) (tx *devnetvm.Transaction, err error) {
	// prepare inputs
	inputs := make(devnetvm.Inputs, len(toBeRefreshed))
	for k, output := range toBeRefreshed {
		inputs[k] = output.Input()
	}
	// prepare outputs
	outputs := make(devnetvm.Outputs, len(toBeRefreshed))
	for k, alias := range toBeRefreshed {
		outputs[k] = alias.NewAliasOutputNext(false)
	}
	// prepare essence
	essence := devnetvm.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		deps.Local.ID(), // pledge both manas to self
		deps.Local.ID(),
		devnetvm.NewInputs(inputs...),
		devnetvm.NewOutputs(outputs...),
	)
	tx = devnetvm.NewTransaction(essence, r.wallet.unlockBlocks(essence))

	// check transaction validity
	if transactionErr := deps.Tangle.Ledger.CheckTransaction(context.Background(), tx); transactionErr != nil {
		return nil, transactionErr
	}

	// check if transaction is too old
	if tx.Essence().Timestamp().Before(clock.SyncedTime().Add(-tangle.MaxReattachmentTimeMin)) {
		return nil, errors.Errorf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangle.MaxReattachmentTimeMin)
	}

	return tx, nil
}

func (r *Refresher) sendTransaction(tx *devnetvm.Transaction) (err error) {
	issueTransaction := func() (*tangle.Message, error) {
		return deps.Tangle.IssuePayload(tx)
	}
	if _, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime); err != nil {
		return err
	}
	return nil
}
