package server

import (
	"io"
	"net"
	"strings"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/goshimmer/packages/txstream/chopper"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil/buffconn"
)

// Connection handles the server-side part of the txstream protocol
type Connection struct {
	bconn                              *buffconn.BufferedConnection
	subscriptions                      atomic.Value
	inTxChan                           chan interface{}
	exitConnChan                       chan struct{}
	receiveConfirmedTransactionClosure *events.Closure
	receiveBookedTransactionClosure    *events.Closure
	receiveMessageClosure              *events.Closure
	messageChopper                     *chopper.Chopper
	atomicLog                          atomic.Value
	ledger                             txstream.Ledger
}

type wrapConfirmedTx *ledgerstate.Transaction
type wrapBookedTx *ledgerstate.Transaction

// Listen starts a TCP listener and starts a Connection for each accepted connection
func Listen(ledger txstream.Ledger, bindAddress string, log *logger.Logger, shutdownSignal <-chan struct{}) {
	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		log.Errorf("failed to start TXStream daemon: %v", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			log.Debugf("accepted connection from %s", conn.RemoteAddr().String())
			go Run(conn, log, ledger, shutdownSignal)
		}
	}()

	<-shutdownSignal

	log.Infof("Detaching TXStream from the Value Tangle..")
	go func() {
		ledger.Detach()
		log.Infof("Detaching TXStream from the Value Tangle..Done")
	}()
}

// Run starts the server-side handling code for an already accepted connection from a client
func Run(conn net.Conn, log *logger.Logger, ledger txstream.Ledger, shutdownSignal <-chan struct{}) {
	c := &Connection{
		bconn:          buffconn.NewBufferedConnection(conn, tangle.MaxMessageSize),
		exitConnChan:   make(chan struct{}),
		messageChopper: chopper.NewChopper(),
		ledger:         ledger,
	}

	c.atomicLog.Store(log) // temporarily until we know the connection ID

	c.subscriptions.Store(make(map[[ledgerstate.AddressLength]byte]bool))

	c.attach()
	defer c.detach()

	select {
	case <-shutdownSignal:
		c.log().Infof("shutdown signal received..")
		_ = c.bconn.Close()

	case <-c.exitConnChan:
		c.log().Infof("closing connection..")
		_ = c.bconn.Close()
	}
}

func (c *Connection) log() *logger.Logger {
	return c.atomicLog.Load().(*logger.Logger)
}

func (c *Connection) setID(id string) {
	c.atomicLog.Store(c.log().Named(id))
	c.log().Infof("client connection id has been set to '%c' for '%s'", id, c.bconn.RemoteAddr().String())
}

func (c *Connection) attach() {
	c.inTxChan = make(chan interface{})

	c.receiveConfirmedTransactionClosure = events.NewClosure(func(tx *ledgerstate.Transaction) {
		c.log().Debugf("on transaction confirmed: %c", tx.ID().String())
		c.inTxChan <- wrapConfirmedTx(tx)
	})
	c.ledger.EventTransactionConfirmed().Attach(c.receiveConfirmedTransactionClosure)

	c.receiveBookedTransactionClosure = events.NewClosure(func(tx *ledgerstate.Transaction) {
		c.log().Debugf("on transaction booked: %c", tx.ID().String())
		c.inTxChan <- wrapBookedTx(tx)
	})
	c.ledger.EventTransactionBooked().Attach(c.receiveBookedTransactionClosure)

	c.receiveMessageClosure = events.NewClosure(func(data []byte) {
		c.processMessageFromClient(data)
	})
	c.bconn.Events.ReceiveMessage.Attach(c.receiveMessageClosure)

	c.log().Debugf("attached txstream")

	// read connection thread
	go func() {
		if err := c.bconn.Read(); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				c.log().Warnw("Permanent error", "err", err)
			}
		}
		close(c.exitConnChan)
	}()

	// read incoming pre-filtered transactions from the ledger
	go func() {
		for vtx := range c.inTxChan {
			switch tvtx := vtx.(type) {
			case wrapConfirmedTx:
				c.processConfirmedTransaction(tvtx)

			case wrapBookedTx:
				c.processBookedTransaction(tvtx)

			default:
				c.log().Panicf("wrong type")
			}
		}
	}()
}

func (c *Connection) detach() {
	c.ledger.EventTransactionConfirmed().Detach(c.receiveConfirmedTransactionClosure)
	c.ledger.EventTransactionBooked().Detach(c.receiveBookedTransactionClosure)
	c.bconn.Events.ReceiveMessage.Detach(c.receiveMessageClosure)

	c.messageChopper.Close()
	close(c.inTxChan)
	_ = c.bconn.Close()
	c.log().Debugf("stopped txstream")
}

func (c *Connection) setSubscriptions(addrs []ledgerstate.Address) (newAddrs []ledgerstate.Address) {
	subscriptions := make(map[[ledgerstate.AddressLength]byte]bool)
	for _, addr := range addrs {
		if !c.isSubscribed(addr) {
			newAddrs = append(newAddrs, addr)
		}
		subscriptions[addr.Array()] = true
	}
	c.subscriptions.Store(subscriptions)
	return
}

func (c *Connection) isSubscribed(addr ledgerstate.Address) bool {
	_, ok := c.subscriptions.Load().(map[[ledgerstate.AddressLength]byte]bool)[addr.Array()]
	return ok
}

func (c *Connection) txSubscribedAddresses(tx *ledgerstate.Transaction) map[[ledgerstate.AddressLength]byte]ledgerstate.Address {
	ret := make(map[[ledgerstate.AddressLength]byte]ledgerstate.Address)
	for _, output := range tx.Essence().Outputs() {
		addr := output.Address()
		if ret[addr.Array()] == nil && c.isSubscribed(addr) {
			ret[addr.Array()] = addr
		}
	}
	return ret
}

// processConfirmedTransaction receives only confirmed transactions
// it parses SC transaction incoming from the ledger. Forwards it to the client if subscribed
func (c *Connection) processConfirmedTransaction(tx *ledgerstate.Transaction) {
	for _, addr := range c.txSubscribedAddresses(tx) {
		c.log().Debugf("confirmed tx -> client -- addr: %s txid: %s", addr.Base58(), tx.ID().String())
		c.pushTransaction(tx.ID(), addr)
	}
}

func (c *Connection) processBookedTransaction(tx *ledgerstate.Transaction) {
	for _, addr := range c.txSubscribedAddresses(tx) {
		c.log().Debugf("booked tx -> client -- addr: %s. txid: %s", addr.Base58(), tx.ID().String())
		c.sendTxInclusionState(tx.ID(), addr, ledgerstate.Pending)
	}
}

func (c *Connection) getTxInclusionState(txid ledgerstate.TransactionID, addr ledgerstate.Address) {
	state, err := c.ledger.GetTxInclusionState(txid)
	if err != nil {
		c.log().Errorf("getTxInclusionState: %v", err)
		return
	}
	c.sendTxInclusionState(txid, addr, state)
}

func (c *Connection) getBacklog(addr ledgerstate.Address) {
	txs := make(map[ledgerstate.TransactionID]bool)
	c.ledger.GetUnspentOutputs(addr, func(out ledgerstate.Output) {
		txs[out.ID().TransactionID()] = true
	})
	for txid := range txs {
		c.pushTransaction(txid, addr)
	}
}

func (c *Connection) postTransaction(tx *ledgerstate.Transaction) {
	if err := c.ledger.PostTransaction(tx); err != nil {
		c.log().Warnf("%v: %c", err, tx.ID().String())
	}
}
