package tangle

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func init() {
	err := parameter.LoadDefaultConfig(false)
	if err != nil {
		log.Fatalf("Failed to initialize config: %s", err)
	}

	logger.InitGlobalLogger(&viper.Viper{})
}

func TestSolidifier(t *testing.T) {
	// show all error messages for tests
	// TODO: adjust logger package

	// start a test node
	node.Start(node.Plugins(PLUGIN))

	// create transactions and chain them together
	transaction1 := value_transaction.New()
	transaction1.SetValue(1)
	require.NoError(t, transaction1.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE))

	transaction2 := value_transaction.New()
	transaction2.SetValue(2)
	transaction2.SetBranchTransactionHash(transaction1.GetHash())
	require.NoError(t, transaction2.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE))

	transaction3 := value_transaction.New()
	transaction3.SetValue(3)
	transaction3.SetBranchTransactionHash(transaction2.GetHash())
	require.NoError(t, transaction3.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE))

	transaction4 := value_transaction.New()
	transaction4.SetValue(4)
	transaction4.SetBranchTransactionHash(transaction3.GetHash())
	require.NoError(t, transaction4.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE))

	// setup event handlers
	var wg sync.WaitGroup
	Events.TransactionSolid.Attach(events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
		t.Log("Tx solidified", transaction.GetValue())
		wg.Done()
	}))

	gossip.Events.RequestTransaction.Attach(events.NewClosure(func(ev *gossip.RequestTransactionEvent) {
		require.Equal(t, transaction3.GetHash(), ev.Hash)
		// return the transaction data
		gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: transaction3.GetBytes()})
	}))

	// issue transactions
	wg.Add(4)

	gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: transaction1.GetBytes()})
	gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: transaction2.GetBytes()})
	// gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: transaction3.GetBytes()})
	gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: transaction4.GetBytes()})

	// wait until all are solid
	wg.Wait()

	// shutdown test node
	node.Shutdown()

}
