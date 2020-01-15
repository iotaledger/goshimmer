package tangle

import (
	"io/ioutil"
	stdlog "log"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/stretchr/testify/require"
)

// use much lower min weight magnitude for the tests
const testMWM = 8

func init() {
	if err := parameter.LoadDefaultConfig(false); err != nil {
		stdlog.Fatalf("Failed to initialize config: %s", err)
	}
	if err := logger.InitGlobalLogger(parameter.NodeConfig); err != nil {
		stdlog.Fatalf("Failed to initialize config: %s", err)
	}
}

func TestTangle(t *testing.T) {
	dir, err := ioutil.TempDir("", "example")
	require.NoError(t, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	parameter.NodeConfig.Set(database.CFG_DIRECTORY, dir)

	// start a test node
	node.Start(node.Plugins(PLUGIN))
	defer node.Shutdown()

	t.Run("ReadTransactionHashesForAddressFromDatabase", func(t *testing.T) {
		tx1 := value_transaction.New()
		tx1.SetAddress("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		tx1.SetTimestamp(uint(time.Now().UnixNano()))
		require.NoError(t, tx1.DoProofOfWork(testMWM))

		tx2 := value_transaction.New()
		tx2.SetTimestamp(uint(time.Now().UnixNano()))
		require.NoError(t, tx2.DoProofOfWork(testMWM))

		transactionReceived(&gossip.TransactionReceivedEvent{Data: tx1.GetBytes()})

		txAddr, err := ReadTransactionHashesForAddressFromDatabase(tx1.GetAddress())
		require.NoError(t, err)
		require.ElementsMatch(t, []trinary.Hash{tx1.GetHash()}, txAddr)
	})

	t.Run("ProofOfWork", func(t *testing.T) {
		tx1 := value_transaction.New()
		tx1.SetTimestamp(uint(time.Now().UnixNano()))
		require.NoError(t, tx1.DoProofOfWork(1))

		tx2 := value_transaction.New()
		tx2.SetTimestamp(uint(time.Now().UnixNano()))
		require.NoError(t, tx2.DoProofOfWork(testMWM))

		var counter int32
		closure := events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
			atomic.AddInt32(&counter, 1)
		})
		Events.TransactionSolid.Attach(closure)
		defer Events.TransactionSolid.Detach(closure)

		transactionReceived(&gossip.TransactionReceivedEvent{Data: tx1.GetBytes()})
		transactionReceived(&gossip.TransactionReceivedEvent{Data: tx2.GetBytes()})

		time.Sleep(100 * time.Millisecond)
		require.EqualValues(t, 1, counter)
	})

	t.Run("Solidifier", func(t *testing.T) {
		transaction1 := value_transaction.New()
		transaction1.SetTimestamp(uint(time.Now().UnixNano()))
		require.NoError(t, transaction1.DoProofOfWork(testMWM))

		transaction2 := value_transaction.New()
		transaction2.SetTimestamp(uint(time.Now().UnixNano()))
		transaction2.SetBranchTransactionHash(transaction1.GetHash())
		require.NoError(t, transaction2.DoProofOfWork(testMWM))

		transaction3 := value_transaction.New()
		transaction3.SetTimestamp(uint(time.Now().UnixNano()))
		transaction3.SetBranchTransactionHash(transaction2.GetHash())
		require.NoError(t, transaction3.DoProofOfWork(testMWM))

		transaction4 := value_transaction.New()
		transaction4.SetTimestamp(uint(time.Now().UnixNano()))
		transaction4.SetBranchTransactionHash(transaction3.GetHash())
		require.NoError(t, transaction4.DoProofOfWork(testMWM))

		var counter int32
		closure := events.NewClosure(func(tx *value_transaction.ValueTransaction) {
			atomic.AddInt32(&counter, 1)
			log.Infof("Transaction solid: hash=%s", tx.GetHash())
		})
		Events.TransactionSolid.Attach(closure)
		defer Events.TransactionSolid.Detach(closure)

		// only transaction3 should be requested
		SetRequester(RequesterFunc(func(hash trinary.Hash) {
			if transaction3.GetHash() == hash {
				// return the transaction data
				transactionReceived(&gossip.TransactionReceivedEvent{Data: transaction3.GetBytes()})
			}
		}))

		transactionReceived(&gossip.TransactionReceivedEvent{Data: transaction1.GetBytes()})
		transactionReceived(&gossip.TransactionReceivedEvent{Data: transaction2.GetBytes()})
		// transactionReceived(&gossip.TransactionReceivedEvent{Data: transaction3.GetBytes()})
		transactionReceived(&gossip.TransactionReceivedEvent{Data: transaction4.GetBytes()})

		time.Sleep(100 * time.Millisecond)
		require.EqualValues(t, 4, counter)
	})
}

// transactionReceived mocks the TransactionReceived event by allowing lower mwm
func transactionReceived(ev *gossip.TransactionReceivedEvent) {
	metaTx := meta_transaction.FromBytes(ev.Data)
	if metaTx.GetWeightMagnitude() < testMWM {
		log.Warnf("invalid weight magnitude: %d / %d", metaTx.GetWeightMagnitude(), testMWM)
		return
	}
	processMetaTransaction(metaTx)
}
