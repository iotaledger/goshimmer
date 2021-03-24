package client

import (
	"net"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
)

// Client represents the client-side connection to a txstream server
type Client struct {
	clientID      string
	log           *logger.Logger
	chSend        chan txstream.Message
	chSubscribe   chan ledgerstate.Address
	chUnsubscribe chan ledgerstate.Address
	shutdown      chan bool
	Events        Events
	wgConnected   sync.WaitGroup
}

// Events contains all events emitted by the Client
type Events struct {
	// MessageReceived is triggered when a message is received from the server
	MessageReceived *events.Event
	// Connected is triggered when the client connects successfully to the server
	Connected *events.Event
}

// DialFunc is a function that performs the TCP connection to the server
type DialFunc func() (addr string, conn net.Conn, err error)

func handleMessageReceived(handler interface{}, params ...interface{}) {
	handler.(func(txstream.Message))(params[0].(txstream.Message))
}

func handleConnected(handler interface{}, params ...interface{}) {
	handler.(func())()
}

// New creates a new client
func New(clientID string, log *logger.Logger, dial DialFunc) *Client {
	n := &Client{
		clientID:      clientID,
		log:           log,
		chSend:        make(chan txstream.Message),
		chSubscribe:   make(chan ledgerstate.Address),
		chUnsubscribe: make(chan ledgerstate.Address),
		shutdown:      make(chan bool),
		Events: Events{
			MessageReceived: events.NewEvent(handleMessageReceived),
			Connected:       events.NewEvent(handleConnected),
		},
	}

	go n.subscriptionsLoop()
	go n.connectLoop(dial)

	return n
}

// Close shuts down the client
func (n *Client) Close() {
	close(n.shutdown)
	n.Events.MessageReceived.DetachAll()
	n.Events.Connected.DetachAll()
}
