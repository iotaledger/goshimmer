package connector

import (
	"io"
	"net"
	"strings"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/waspconn"
	"github.com/iotaledger/goshimmer/packages/waspconn/chopper"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil/buffconn"
)

type WaspConnector struct {
	bconn                              *buffconn.BufferedConnection
	subscriptions                      atomic.Value
	inTxChan                           chan interface{}
	exitConnChan                       chan struct{}
	receiveConfirmedTransactionClosure *events.Closure
	receiveBookedTransactionClosure    *events.Closure
	receiveWaspMessageClosure          *events.Closure
	messageChopper                     *chopper.Chopper
	atomicLog                          atomic.Value
	ledger                             waspconn.Ledger
}

type wrapConfirmedTx *ledgerstate.Transaction
type wrapBookedTx *ledgerstate.Transaction

func Run(conn net.Conn, log *logger.Logger, ledger waspconn.Ledger, shutdownSignal <-chan struct{}) {
	wconn := &WaspConnector{
		bconn:          buffconn.NewBufferedConnection(conn, tangle.MaxMessageSize),
		exitConnChan:   make(chan struct{}),
		messageChopper: chopper.NewChopper(),
		ledger:         ledger,
	}

	wconn.atomicLog.Store(log) // temporarily until we know the connection ID

	wconn.subscriptions.Store(make(map[[ledgerstate.AddressLength]byte]bool))

	wconn.attach()
	defer wconn.detach()

	select {
	case <-shutdownSignal:
		wconn.log().Infof("shutdown signal received..")
		_ = wconn.bconn.Close()

	case <-wconn.exitConnChan:
		wconn.log().Infof("closing connection..")
		_ = wconn.bconn.Close()
	}
}

func (wconn *WaspConnector) log() *logger.Logger {
	return wconn.atomicLog.Load().(*logger.Logger)
}

func (wconn *WaspConnector) SetId(id string) {
	wconn.atomicLog.Store(wconn.log().Named(id))
	wconn.log().Infof("wasp connection id has been set to '%s' for '%s'", id, wconn.bconn.RemoteAddr().String())
}

func (wconn *WaspConnector) attach() {
	wconn.inTxChan = make(chan interface{})

	wconn.receiveConfirmedTransactionClosure = events.NewClosure(func(tx *ledgerstate.Transaction) {
		wconn.log().Debugf("on transaction confirmed: %s", tx.ID().String())
		wconn.inTxChan <- wrapConfirmedTx(tx)
	})
	wconn.ledger.EventTransactionConfirmed().Attach(wconn.receiveConfirmedTransactionClosure)

	wconn.receiveBookedTransactionClosure = events.NewClosure(func(tx *ledgerstate.Transaction) {
		wconn.log().Debugf("on transaction booked: %s", tx.ID().String())
		wconn.inTxChan <- wrapBookedTx(tx)
	})
	wconn.ledger.EventTransactionBooked().Attach(wconn.receiveBookedTransactionClosure)

	wconn.receiveWaspMessageClosure = events.NewClosure(func(data []byte) {
		wconn.processMsgDataFromWasp(data)
	})
	wconn.bconn.Events.ReceiveMessage.Attach(wconn.receiveWaspMessageClosure)

	wconn.log().Debugf("attached waspconn")

	// read connection thread
	go func() {
		if err := wconn.bconn.Read(); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				wconn.log().Warnw("Permanent error", "err", err)
			}
		}
		close(wconn.exitConnChan)
	}()

	// read incoming pre-filtered transactions from node
	go func() {
		for vtx := range wconn.inTxChan {
			switch tvtx := vtx.(type) {
			case wrapConfirmedTx:
				wconn.processConfirmedTransactionFromNode(tvtx)

			case wrapBookedTx:
				wconn.processBookedTransactionFromNode(tvtx)

			default:
				wconn.log().Panicf("wrong type")
			}
		}
	}()
}

func (wconn *WaspConnector) detach() {
	wconn.ledger.EventTransactionConfirmed().Detach(wconn.receiveConfirmedTransactionClosure)
	wconn.ledger.EventTransactionBooked().Detach(wconn.receiveBookedTransactionClosure)
	wconn.bconn.Events.ReceiveMessage.Detach(wconn.receiveWaspMessageClosure)

	wconn.messageChopper.Close()
	close(wconn.inTxChan)
	_ = wconn.bconn.Close()
	wconn.log().Debugf("stopped waspconn")
}

func (wconn *WaspConnector) setSubscriptions(addrs []*ledgerstate.AliasAddress) (newAddrs []*ledgerstate.AliasAddress) {
	subscriptions := make(map[[ledgerstate.AddressLength]byte]bool)
	for _, addr := range addrs {
		if !wconn.isSubscribed(addr) {
			newAddrs = append(newAddrs, addr)
		}
		subscriptions[addr.Array()] = true
	}
	wconn.subscriptions.Store(subscriptions)
	return
}

func (wconn *WaspConnector) isSubscribed(addr *ledgerstate.AliasAddress) bool {
	_, ok := wconn.subscriptions.Load().(map[[ledgerstate.AddressLength]byte]bool)[addr.Array()]
	return ok
}

func isRequestOrChainOutput(out ledgerstate.Output) bool {
	switch out.(type) {
	case *ledgerstate.ChainOutput:
		return true
	case *ledgerstate.ExtendedLockedOutput:
		return true
	}
	return false
}

func (wconn *WaspConnector) txSubscribedAddresses(tx *ledgerstate.Transaction) []*ledgerstate.AliasAddress {
	ret := make([]*ledgerstate.AliasAddress, 0)
	for _, output := range tx.Essence().Outputs() {
		if !isRequestOrChainOutput(output) {
			continue
		}
		addr, ok := output.Address().(*ledgerstate.AliasAddress)
		if !ok {
			continue
		}
		if wconn.isSubscribed(addr) {
			ret = append(ret, addr)
		}
	}
	return ret
}

// processConfirmedTransactionFromNode receives only confirmed transactions
// it parses SC transaction incoming from the node. Forwards it to Wasp if subscribed
func (wconn *WaspConnector) processConfirmedTransactionFromNode(tx *ledgerstate.Transaction) {
	for _, addr := range wconn.txSubscribedAddresses(tx) {
		wconn.log().Debugf("confirmed tx -> Wasp -- addr: %s txid: %s", addr.Base58(), tx.ID().String())
		wconn.pushTransaction(tx.ID(), addr)
	}
}

func (wconn *WaspConnector) processBookedTransactionFromNode(tx *ledgerstate.Transaction) {
	addrs := wconn.txSubscribedAddresses(tx)
	if len(addrs) == 0 {
		return
	}
	txid := tx.ID()
	wconn.log().Debugf("booked tx -> Wasp. txid: %s", tx.ID().String())
	wconn.sendTxInclusionStateToWasp(txid, ledgerstate.Pending)
}

func (wconn *WaspConnector) getTxInclusionState(txid ledgerstate.TransactionID) {
	state, err := wconn.ledger.GetTxInclusionState(txid)
	if err != nil {
		wconn.log().Errorf("getTxInclusionState: %v", err)
		return
	}
	wconn.sendTxInclusionStateToWasp(txid, state)
}

func (wconn *WaspConnector) getBacklog(addr *ledgerstate.AliasAddress) {
	txs := make(map[ledgerstate.TransactionID]bool)
	wconn.ledger.GetUnspentOutputs(addr, func(out ledgerstate.Output) {
		txid := out.ID().TransactionID()
		if isRequestOrChainOutput(out) {
			txs[txid] = true
		}
	})
	for txid := range txs {
		wconn.pushTransaction(txid, addr)
	}
}

func (wconn *WaspConnector) postTransaction(tx *ledgerstate.Transaction) {
	if err := wconn.ledger.PostTransaction(tx); err != nil {
		wconn.log().Warnf("%v: %s", err, tx.ID().String())
	}
}
