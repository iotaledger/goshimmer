package client

import (
	"net"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

// Client represents the client-side connection to a txstream server.
type Client struct {
	clientID      string
	log           *logger.Logger
	chSend        chan txstream.Message
	chSubscribe   chan ledgerstate.Address
	chUnsubscribe chan ledgerstate.Address
	shutdown      chan bool
	Events        Events
}

// Events contains all events emitted by the Client.
type Events struct {
	// TransactionReceived is triggered when a transaction message is received from the server
	TransactionReceived *events.Event
	// InclusionStateReceived is triggered when an inclusion state message is received from the server
	InclusionStateReceived *events.Event
	// OutputReceived is triggered whenever individual output is received
	OutputReceived *events.Event
	// UnspentAliasOutputReceived is triggered whenever an unspent AliasOutput is received
	UnspentAliasOutputReceived *events.Event
	// Connected is triggered when the client connects successfully to the server
	Connected *events.Event
}

// DialFunc is a function that performs the TCP connection to the server.
type DialFunc func() (addr string, conn net.Conn, err error)

func handleTransactionReceived(handler interface{}, params ...interface{}) {
	handler.(func(*txstream.MsgTransaction))(params[0].(*txstream.MsgTransaction))
}

func handleOutputReceived(handler interface{}, params ...interface{}) {
	handler.(func(output *txstream.MsgOutput))(params[0].(*txstream.MsgOutput))
}

func handleUnspentAliasOutputReceived(handler interface{}, params ...interface{}) {
	handler.(func(output *txstream.MsgUnspentAliasOutput))(params[0].(*txstream.MsgUnspentAliasOutput))
}

func handleInclusionStateReceived(handler interface{}, params ...interface{}) {
	handler.(func(*txstream.MsgTxGoF))(params[0].(*txstream.MsgTxGoF))
}

func handleConnected(handler interface{}, params ...interface{}) {
	handler.(func())()
}

// New creates a new client.
func New(clientID string, log *logger.Logger, dial DialFunc) *Client {
	n := &Client{
		clientID:      clientID,
		log:           log,
		chSend:        make(chan txstream.Message),
		chSubscribe:   make(chan ledgerstate.Address),
		chUnsubscribe: make(chan ledgerstate.Address),
		shutdown:      make(chan bool),
		Events: Events{
			TransactionReceived:        events.NewEvent(handleTransactionReceived),
			InclusionStateReceived:     events.NewEvent(handleInclusionStateReceived),
			OutputReceived:             events.NewEvent(handleOutputReceived),
			UnspentAliasOutputReceived: events.NewEvent(handleUnspentAliasOutputReceived),
			Connected:                  events.NewEvent(handleConnected),
		},
	}

	go n.subscriptionsLoop()
	go n.connectLoop(dial)

	return n
}

// Close shuts down the client.
func (n *Client) Close() {
	close(n.shutdown)
	n.Events.TransactionReceived.DetachAll()
	n.Events.InclusionStateReceived.DetachAll()
	n.Events.OutputReceived.DetachAll()
	n.Events.UnspentAliasOutputReceived.DetachAll()
	n.Events.Connected.DetachAll()
}
