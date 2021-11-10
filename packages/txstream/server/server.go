package server

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil/buffconn"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/goshimmer/packages/txstream/chopper"
)

// Connection handles the server-side part of the txstream protocol.
type Connection struct {
	bconn         *buffconn.BufferedConnection
	chopper       *chopper.Chopper
	subscriptions map[[ledgerstate.AddressLength]byte]bool
	ledger        txstream.Ledger
	log           *logger.Logger
}

type (
	wrapBookedTx *ledgerstate.Transaction
)

const rcvClientIDTimeout = 5 * time.Second

// Listen starts a TCP listener and starts a Connection for each accepted connection.
func Listen(ledger txstream.Ledger, bindAddress string, log *logger.Logger, shutdownSignal <-chan struct{}) error {
	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		return fmt.Errorf("failed to start TXStream daemon: %w", err)
	}

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

	go func() {
		defer listener.Close()

		<-shutdownSignal

		log.Infof("Detaching TXStream from the Value Tangle..")
		log.Infof("Detaching TXStream from the Value Tangle..Done")
	}()

	return nil
}

// Run starts the server-side handling code for an already accepted connection from a client.
func Run(conn net.Conn, log *logger.Logger, ledger txstream.Ledger, shutdownSignal <-chan struct{}) {
	c := &Connection{
		bconn:         buffconn.NewBufferedConnection(conn, tangle.MaxMessageSize),
		chopper:       chopper.NewChopper(),
		subscriptions: make(map[[ledgerstate.AddressLength]byte]bool),
		ledger:        ledger,
		log:           log,
	}

	defer c.bconn.Close()
	defer c.chopper.Close()

	bconnDataReceived, bconnClosed := c.bconnReadLoop()

	// expect first message received from client == MsgSetID
	select {
	case data := <-bconnDataReceived:
		id, err := c.receiveClientID(data)
		if err != nil {
			c.log.Errorf("first message from client: %v", err)
			return
		}
		c.log = c.log.Named(id)
		c.log.Infof("client connection id has been set to '%s' for '%s'", id, c.bconn.RemoteAddr().String())
	case <-shutdownSignal:
		c.log.Infof("shutdown signal received")
		return
	case <-bconnClosed:
		c.log.Errorf("connection lost")
		return
	case <-time.After(rcvClientIDTimeout):
		c.log.Errorf("timeout receiving client ID")
		return
	}

	txFromLedgerQueue := make(chan interface{})

	{
		cl := events.NewClosure(func(tx *ledgerstate.Transaction) {
			c.log.Debugf("on transaction booked: %s", tx.ID().Base58())
			txFromLedgerQueue <- wrapBookedTx(tx)
		})
		c.ledger.EventTransactionBooked().Attach(cl)
		defer c.ledger.EventTransactionBooked().Detach(cl)
	}

	c.log.Debugf("started txStream")
	defer c.log.Debugf("stopped txStream")

	// single-threaded r/w loop
	for {
		select {
		case tx := <-txFromLedgerQueue:
			switch tx := tx.(type) {
			case wrapBookedTx:
				c.processBookedTransaction(tx)
			default:
				c.log.Panicf("wrong type")
			}
		case data := <-bconnDataReceived:
			if err := c.processMessageFromClient(data); err != nil {
				c.log.Errorf("processMessageFromClient: %v", err)
			}
		case <-shutdownSignal:
			c.log.Infof("shutdown signal received")
			return
		case <-bconnClosed:
			c.log.Errorf("connection lost")
			return
		}
	}
}

func (c *Connection) bconnReadLoop() (chan []byte, chan bool) {
	bconnDataReceived := make(chan []byte)
	bconnClosed := make(chan bool)

	// bconn read loop
	go func() {
		{
			cl := events.NewClosure(func() { close(bconnClosed) })
			c.bconn.Events.Close.Attach(cl)
			defer c.bconn.Events.Close.Detach(cl)
		}

		{
			cl := events.NewClosure(func(data []byte) {
				// data slice is from internal buffconn buffer
				d := make([]byte, len(data))
				copy(d, data)
				bconnDataReceived <- d
			})
			c.bconn.Events.ReceiveMessage.Attach(cl)
			defer c.bconn.Events.ReceiveMessage.Detach(cl)
		}

		if err := c.bconn.Read(); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				c.log.Warnw("bconn read error", "err", err)
			}
		}
	}()

	return bconnDataReceived, bconnClosed
}

func (c *Connection) setSubscriptions(addrs []ledgerstate.Address) (newAddrs []ledgerstate.Address) {
	subscriptions := make(map[[ledgerstate.AddressLength]byte]bool)
	for _, addr := range addrs {
		if !c.isSubscribed(addr) {
			newAddrs = append(newAddrs, addr)
		}
		subscriptions[addr.Array()] = true
	}
	c.subscriptions = subscriptions
	return
}

func (c *Connection) isSubscribed(addr ledgerstate.Address) bool {
	_, ok := c.subscriptions[addr.Array()]
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
// it parses SC transaction incoming from the ledger. Forwards it to the client if subscribed.
func (c *Connection) processConfirmedTransaction(tx *ledgerstate.Transaction) {
	for _, addr := range c.txSubscribedAddresses(tx) {
		c.log.Debugf("confirmed tx -> client -- addr: %s txid: %s", addr.Base58(), tx.ID().Base58())
		c.pushTransaction(tx.ID(), addr)
	}
}

func (c *Connection) processBookedTransaction(tx *ledgerstate.Transaction) {
	for _, addr := range c.txSubscribedAddresses(tx) {
		c.log.Debugf("booked tx -> client -- addr: %s. txid: %s", addr.Base58(), tx.ID().Base58())
		c.sendTxInclusionState(tx.ID(), addr, gof.Low)
	}
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
		c.log.Debugf("%v: %s", err, tx.ID().Base58())
	}
}
