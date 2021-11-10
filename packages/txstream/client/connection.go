package client

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/backoff"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/netutil/buffconn"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/goshimmer/packages/txstream/chopper"
)

const (
	dialRetries  = 10
	backoffDelay = 500 * time.Millisecond
	retryAfter   = 8 * time.Second
)

// retry net.Dial once, on fail after 0.5s.
var dialRetryPolicy = backoff.ConstantBackOff(backoffDelay).With(backoff.MaxRetries(dialRetries))

func (n *Client) connectLoop(dial DialFunc) {
	msgChopper := chopper.NewChopper()
	for {
		retry := n.connect(dial, msgChopper)
		if !retry {
			return
		}
		n.log.Infof("disconnected from server - will retry reconnecting after %v", retryAfter)
		select {
		case <-n.shutdown:
			return
		case <-time.After(retryAfter):
		}
	}
}

// dials outbound address and established connection.
func (n *Client) connect(dial DialFunc, msgChopper *chopper.Chopper) bool {
	var addr string
	var conn net.Conn
	if err := backoff.Retry(dialRetryPolicy, func() error {
		var err error
		addr, conn, err = dial()
		if err != nil {
			return fmt.Errorf("can't connect with the server: %v", err)
		}
		return nil
	}); err != nil {
		n.log.Warn(err)
		// retry
		return true
	}

	bconn := buffconn.NewBufferedConnection(conn, tangle.MaxMessageSize)
	defer func() {
		n.log.Debugf("closing bconn")
		bconn.Close()
	}()
	n.Events.Connected.Trigger()

	n.log.Debugf("established connection with server at %s", addr)

	dataReceived := make(chan []byte)
	{
		cl := events.NewClosure(func(data []byte) {
			// data slice is from internal buffconn buffer
			d := make([]byte, len(data))
			copy(d, data)
			dataReceived <- d
		})
		bconn.Events.ReceiveMessage.Attach(cl)
		defer bconn.Events.ReceiveMessage.Detach(cl)
	}

	connectionClosed := make(chan bool)
	{
		cl := events.NewClosure(func() {
			n.log.Errorf("lost connection with %s", addr)
			close(connectionClosed)
		})
		bconn.Events.Close.Attach(cl)
		defer bconn.Events.Close.Detach(cl)
	}

	// read loop
	go func() {
		if err := bconn.Read(); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				n.log.Warnw("bconn read error", "err", err)
			}
		}
	}()

	// send client ID
	if err := n.send(&txstream.MsgSetID{ClientID: n.clientID}, bconn, msgChopper); err != nil {
		n.log.Errorf("sending client ID to server: %v", err)
	}

	// r/w loop
	for {
		select {
		case msg := <-n.chSend:
			if err := n.send(msg, bconn, msgChopper); err != nil {
				n.log.Errorf("sending message to server (%T): %v", msg, err)
			}
		case d := <-dataReceived:
			if err := n.decodeReceivedMessage(d, msgChopper); err != nil {
				n.log.Errorf("decoding message from server: %v", err)
			}
		case <-n.shutdown:
			return false
		case <-connectionClosed:
			return true // retry
		}
	}
}

func (n *Client) decodeReceivedMessage(data []byte, msgChopper *chopper.Chopper) error {
	msg, err := txstream.DecodeMsg(data, txstream.FlagServerToClient)
	if err != nil {
		return fmt.Errorf("txstream.DecodeMsg: %w", err)
	}

	switch msg := msg.(type) {
	case *txstream.MsgChunk:
		finalData, err := msgChopper.IncomingChunk(msg.Data, tangle.MaxMessageSize, txstream.ChunkMessageHeaderSize)
		if err != nil {
			return fmt.Errorf("receiving msgchunk: %w", err)
		}
		if finalData != nil {
			return n.decodeReceivedMessage(finalData, msgChopper)
		}
	case *txstream.MsgTransaction:
		n.log.Debugf("received message from server: %T", msg)
		n.Events.TransactionReceived.Trigger(msg)

	case *txstream.MsgTxGoF:
		n.log.Debugf("received message from server: %T", msg)
		n.Events.InclusionStateReceived.Trigger(msg)

	case *txstream.MsgOutput:
		n.log.Debugf("received message from server: %T", msg)
		n.Events.OutputReceived.Trigger(msg)

	case *txstream.MsgUnspentAliasOutput:
		n.log.Debugf("received message from server: %T", msg)
		n.Events.UnspentAliasOutputReceived.Trigger(msg)

	default:
		n.log.Errorf("received unknkwn message from server: %T", msg)
	}
	return nil
}

// sendMessage is a thread-safe request to send a message to the server.
func (n *Client) sendMessage(msg txstream.Message) {
	n.chSend <- msg
}

// send writes a message into the server connection.
func (n *Client) send(msg txstream.Message, bconn *buffconn.BufferedConnection, msgChopper *chopper.Chopper) error {
	n.log.Debugf("sending message to server: %T", msg)
	data := txstream.EncodeMsg(msg)
	choppedData, chopped, err := msgChopper.ChopData(data, tangle.MaxMessageSize, txstream.ChunkMessageHeaderSize)
	if err != nil {
		return err
	}
	if !chopped {
		_, err = bconn.Write(data)
		return err
	}
	for _, piece := range choppedData {
		if _, err = bconn.Write(txstream.EncodeMsg(&txstream.MsgChunk{Data: piece})); err != nil {
			return err
		}
	}
	return nil
}
