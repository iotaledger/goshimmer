package client

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/goshimmer/packages/txstream/server"
	"github.com/iotaledger/goshimmer/packages/txstream/utxodbledger"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	creatorIndex      = 2
	stateControlIndex = 3
)

var log = initLog()

func initLog() *logger.Logger {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return log.Sugar()
}

func start(t *testing.T) (*utxodbledger.UtxoDBLedger, *Client) {
	t.Helper()

	ledger := utxodbledger.New()
	t.Cleanup(ledger.Detach)

	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	dial := DialFunc(func() (string, net.Conn, error) {
		conn1, conn2 := net.Pipe()
		go server.Run(conn2, log.Named("txstream/server"), ledger, done)
		return "pipe", conn1, nil
	})

	n := New("test", log.Named("txstream/client"), dial)
	t.Cleanup(n.Close)

	ok := n.WaitForConnection(10 * time.Second)
	require.True(t, ok)

	return ledger, n
}

func send(t *testing.T, n *Client, sendMsg func(), rcv func(msg txstream.Message) bool) {
	t.Helper()

	done := make(chan bool)

	var wgSend sync.WaitGroup
	wgSend.Add(1)

	closureTransactionReceived := events.NewClosure(func(msg *txstream.MsgTransaction) {
		t.Logf("received 'transaction' from txstream: %s", msg.Tx.ID().Base58())
		wgSend.Wait()
		if rcv(msg) {
			close(done)
		}
	})

	closureInlucsionStateReceived := events.NewClosure(func(msg *txstream.MsgTxInclusionState) {
		t.Logf("received 'inlcusion state' from txstream: %s: %s", msg.TxID.Base58(), msg.State.String())
		wgSend.Wait()
		if rcv(msg) {
			close(done)
		}
	})

	n.Events.TransactionReceived.Attach(closureTransactionReceived)
	defer n.Events.TransactionReceived.Detach(closureTransactionReceived)

	n.Events.InclusionStateReceived.Attach(closureInlucsionStateReceived)
	defer n.Events.InclusionStateReceived.Detach(closureInlucsionStateReceived)

	sendMsg()
	wgSend.Done()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout")
	}
}

func createAliasChain(t *testing.T, u *utxodbledger.UtxoDBLedger, creatorIndex int, stateControlIndex int, balances map[ledgerstate.Color]uint64) (*ledgerstate.Transaction, *ledgerstate.AliasAddress) {
	t.Helper()

	creatorKP, creatorAddr := u.NewKeyPairByIndex(creatorIndex)
	err := u.RequestFunds(creatorAddr)
	require.NoError(t, err)

	_, addrStateControl := u.NewKeyPairByIndex(stateControlIndex)
	outputs := u.GetAddressOutputs(creatorAddr)
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewAliasMint(balances, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(creatorAddr, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(creatorKP)
	require.NoError(t, err)

	err = u.PostTransaction(tx)
	require.NoError(t, err)

	chainOutput, err := utxoutil.GetSingleChainedAliasOutput(tx.Essence())
	require.NoError(t, err)
	chainAddress := chainOutput.GetAliasAddress()
	t.Logf("chain address: %s", chainAddress.Base58())

	return tx, chainAddress
}

func TestRequestBacklog(t *testing.T) {
	ledger, n := start(t)

	tx, chainAddress := createAliasChain(t, ledger, creatorIndex, stateControlIndex, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100})

	// request backlog for chainAddress
	var resp *txstream.MsgTransaction
	send(t, n,
		func() {
			n.RequestBacklog(chainAddress)
		},
		func(msg txstream.Message) bool {
			switch msg := msg.(type) {
			case *txstream.MsgTransaction:
				resp = msg
				return true
			}
			return false
		},
	)

	// assert response message
	require.EqualValues(t, chainAddress.Base58(), resp.Address.Base58())

	_, creatorAddr := ledger.NewKeyPairByIndex(creatorIndex)
	t.Logf("creator address: %s", creatorAddr.Base58())

	require.Equal(t, tx.ID(), resp.Tx.ID())

	chainOutput, err := utxoutil.GetSingleChainedAliasOutput(resp.Tx.Essence())
	require.NoError(t, err)
	require.EqualValues(t, chainAddress.Base58(), chainOutput.Address().Base58())
}

func postRequest(t *testing.T, u *utxodbledger.UtxoDBLedger, fromIndex int, chainAddress *ledgerstate.AliasAddress) *ledgerstate.Transaction {
	kp, addr := u.NewKeyPairByIndex(fromIndex)

	outs := u.GetAddressOutputs(addr)

	txb := utxoutil.NewBuilder(outs...)
	err := txb.AddExtendedOutputConsume(chainAddress, []byte{1, 3, 3, 7}, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 1})
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(kp)
	require.NoError(t, err)

	err = u.PostTransaction(tx)
	require.NoError(t, err)

	return tx
}

func TestPostRequest(t *testing.T) {
	ledger, n := start(t)

	createTx, chainAddress := createAliasChain(t, ledger, creatorIndex, stateControlIndex, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100})

	reqTx := postRequest(t, ledger, 2, chainAddress)

	// request backlog for chainAddress
	seen := make(map[ledgerstate.TransactionID]bool)
	send(t, n,
		func() {
			n.RequestBacklog(chainAddress)
		},
		func(msg txstream.Message) bool {
			switch msg := msg.(type) {
			case *txstream.MsgTransaction:
				t.Logf("seen tx %s", msg.Tx.ID().Base58())
				seen[msg.Tx.ID()] = true
				if len(seen) == 2 {
					return true
				}
			}
			return false
		},
	)

	require.Equal(t, 2, len(seen))
	require.True(t, seen[createTx.ID()])
	require.True(t, seen[reqTx.ID()])
}

func TestRequestInclusionLevel(t *testing.T) {
	ledger, n := start(t)
	createTx, chainAddress := createAliasChain(t, ledger, creatorIndex, stateControlIndex, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100})

	// request inclusion level
	var resp *txstream.MsgTxInclusionState
	send(t, n,
		func() {
			n.RequestTxInclusionState(chainAddress, createTx.ID())
		},
		func(msg txstream.Message) bool {
			if msg, ok := msg.(*txstream.MsgTxInclusionState); ok {
				resp = msg
				return true
			}
			return false
		},
	)

	require.EqualValues(t, ledgerstate.Confirmed, resp.State)
}

func TestSubscribe(t *testing.T) {
	ledger, n := start(t)
	_, chainAddress := createAliasChain(t, ledger, creatorIndex, stateControlIndex, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100})

	// subscribe to chain address
	n.Subscribe(chainAddress)

	// post a request to chain, expect to receive notification
	var reqTx *ledgerstate.Transaction
	var txMsg *txstream.MsgTransaction

	send(t, n,
		func() {
			reqTx = postRequest(t, ledger, 2, chainAddress)
		},
		func(msg txstream.Message) bool {
			switch msg := msg.(type) {
			case *txstream.MsgTransaction:
				t.Logf("received tx %s", msg.Tx.ID().Base58())
				if msg.Tx.ID() == reqTx.ID() {
					txMsg = msg
					return true
				}
			}
			return false
		},
	)
	require.EqualValues(t, txMsg.Tx.ID(), reqTx.ID())
}
