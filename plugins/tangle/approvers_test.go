package tangle

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

func TestSolidifier(t *testing.T) {
	// initialize plugin
	configureDatabase(nil)
	configureSolidifier(nil)

	// create transactions and chain them together
	transaction1 := NewTransaction(nil)
	transaction1.SetNonce(ternary.Trinary("99999999999999999999999999A"))
	transaction2 := NewTransaction(nil)
	transaction2.SetBranchTransactionHash(transaction1.GetHash())
	transaction3 := NewTransaction(nil)
	transaction3.SetBranchTransactionHash(transaction2.GetHash())
	transaction4 := NewTransaction(nil)
	transaction4.SetBranchTransactionHash(transaction3.GetHash())

	// setup event handlers
	var wg sync.WaitGroup
	Events.TransactionSolid.Attach(events.NewClosure(func(transaction *Transaction) {
		wg.Done()
	}))

	// issue transactions
	wg.Add(4)
	gossip.Events.ReceiveTransaction.Trigger(transaction1.Flush())
	gossip.Events.ReceiveTransaction.Trigger(transaction2.Flush())
	gossip.Events.ReceiveTransaction.Trigger(transaction3.Flush())
	gossip.Events.ReceiveTransaction.Trigger(transaction4.Flush())

	// wait until all are solid
	wg.Wait()
}
