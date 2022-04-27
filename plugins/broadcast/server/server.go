package server

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/netutil/buffconn"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type connection struct {
	bufferedConn *buffconn.BufferedConnection
	log          *node.Plugin // Just used for logging.
	active       bool
}

var (
	connectionList      = [256]connection{}
	connectionListMutex sync.RWMutex
	index               atomic.Uint32
)

const indexThreshold = 250

// Listen starts a TCP listener and starts a Connection for each accepted connection.
func Listen(bindAddress string, log *node.Plugin, shutdownSignal <-chan struct{}) error {
	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		return fmt.Errorf("failed to start Broadcast daemon: %w", err)
	}

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				log.LogInfof("Couldn't accept connection: %s", acceptErr)
				return
			}
			log.LogDebugf("Started connection: %s", conn.RemoteAddr().String())
			go handleConnection(conn, log, shutdownSignal)
		}
	}()

	go func() {
		defer func(listener net.Listener) {
			if listener.Close() != nil {
				log.LogInfof("Error closing listener: %s", err)
			}
		}(listener)

		<-shutdownSignal

		log.LogInfof("Closing Broadcast server...")
		idx := int(index.Load())
		for i := 0; i < idx; i++ {
			connectionList[i].active = false
		}
		removeInactiveConnections()
		log.LogInfof("Closing Broadcast server... done")
	}()

	return nil
}

func handleConnection(conn net.Conn, log *node.Plugin, shutdownSignal <-chan struct{}) {
	connectionListMutex.Lock()

	idx := int(index.Load())
	connectionList[idx] = connection{
		bufferedConn: buffconn.NewBufferedConnection(conn, tangle.MaxMessageSize),
		log:          log,
		active:       true,
	}
	index.Inc()
	connectionListMutex.Unlock()

	bufferedConnDataReceived, bufferedConnClosed := connectionList[idx].readLoop()

	select {
	case data := <-bufferedConnDataReceived:
		// No input required. For debugging it will be printed.
		connectionList[idx].log.LogDebugf("Data received:%v", data)
	case <-shutdownSignal:
		connectionList[idx].log.LogInfof("Shutdown signal received")
		connectionList[idx].active = false
	case <-bufferedConnClosed:
		connectionList[idx].log.LogDebugf("Connection lost")
		connectionList[idx].active = false
	}
}

func (connection *connection) readLoop() (chan []byte, chan bool) {
	bufferedConnDataReceived := make(chan []byte)
	bufferedConnClosed := make(chan bool)

	go func() {
		connectionClosedClosure := event.NewClosure[*buffconn.CloseEvent](func(_ *buffconn.CloseEvent) { close(bufferedConnClosed) })
		connection.bufferedConn.Events.Close.Hook(connectionClosedClosure)
		defer connection.bufferedConn.Events.Close.Detach(connectionClosedClosure)

		connectionDataReceivedClosure := event.NewClosure[*buffconn.ReceiveMessageEvent](func(event *buffconn.ReceiveMessageEvent) {
			d := make([]byte, len(event.Data))
			copy(d, event.Data)
			bufferedConnDataReceived <- d
		})

		connection.bufferedConn.Events.ReceiveMessage.Hook(connectionDataReceivedClosure)
		defer connection.bufferedConn.Events.ReceiveMessage.Detach(connectionDataReceivedClosure)

		if err := connection.bufferedConn.Read(); err != nil {
			if err != io.EOF && errors.Is(err, net.ErrClosed) {
				connection.log.LogDebugf("Buffered connection read error", "err", err)
				connection.active = false
			}
		}
	}()

	return bufferedConnDataReceived, bufferedConnClosed
}

// Broadcast sends data to all active connections.
func Broadcast(data []byte) {
	connectionListMutex.Lock()
	defer connectionListMutex.Unlock()

	idx := int(index.Load())
	for i := 0; i < idx; i++ {
		if !connectionList[i].active {
			continue
		}
		if _, err := connectionList[i].bufferedConn.Write(data); err != nil {
			connectionList[i].log.LogInfof("Error writing on connection: %s", err)
			connectionList[i].active = false
		}
	}
	// Tidy up array of unused connections.
	removeInactiveConnections()
}

func removeInactiveConnections() {
	idx := int(index.Load())
	if idx >= indexThreshold {
		newIndex := 0
		newConnectionList := [256]connection{}
		for i := 0; i < idx; i++ {
			if connectionList[i].active {
				newConnectionList[newIndex] = connectionList[i]
				newIndex++
			}
		}
		index.Store(uint32(newIndex))
		connectionList = newConnectionList
	}
}
