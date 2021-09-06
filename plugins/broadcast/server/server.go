package server

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/node"
	"sync"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/netutil/buffconn"

	"fmt"
	"io"
	"net"
)

type connection struct {
	bufferedConn *buffconn.BufferedConnection
	log          *node.Plugin //Is just used for logging
	active       bool
}

var (
	connectionList      = [256]connection{}
	connectionListMutex sync.RWMutex
	index               atomic.Uint32
)

// Listen starts a TCP listener and starts a Connection for each accepted connection
func Listen(bindAddress string, log *node.Plugin, shutdownSignal <-chan struct{}) error {
	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		return fmt.Errorf("failed to start Broadcast daemon: %w", err)
	}

	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				if connection != nil {
					log.LogInfof("Couldn't accept connection: %s", err)
				}
				return
			}
			log.LogDebugf("Started connection: %s", connection.RemoteAddr().String())
			go handleConnection(connection, log, shutdownSignal)
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
		//No input required. For debugging it will be printed
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
		{
			connectionClosedClosure := events.NewClosure(func() { close(bufferedConnClosed) })
			connection.bufferedConn.Events.Close.Attach(connectionClosedClosure)
			defer connection.bufferedConn.Events.Close.Detach(connectionClosedClosure)
		}

		{
			connectionDataReceivedClosure := events.NewClosure(func(data []byte) {
				d := make([]byte, len(data))
				copy(d, data)
				bufferedConnDataReceived <- d
			})
			connection.bufferedConn.Events.ReceiveMessage.Attach(connectionDataReceivedClosure)
			defer connection.bufferedConn.Events.ReceiveMessage.Detach(connectionDataReceivedClosure)
		}

		if err := connection.bufferedConn.Read(); err != nil {
			if err != io.EOF && errors.Is(err, net.ErrClosed) {
				connection.log.LogDebugf("Buffered connection read error", "err", err)
				connection.active = false
			}
		}
	}()

	return bufferedConnDataReceived, bufferedConnClosed
}

func Broadcast(data []byte) {
	connectionListMutex.Lock()
	defer connectionListMutex.Unlock()

	idx := int(index.Load())
	for i := 0; i < idx; i++ {
		if connectionList[i].active {
			if _, err := connectionList[i].bufferedConn.Write(data); err != nil {
				connectionList[i].log.LogInfof("Error writing on connection: %s", err)
				connectionList[i].active = false
			}
		}
	}
	//Tidy up array of unused connections
	removeInactiveConnections()
}

func removeInactiveConnections() {
	idx := int(index.Load())
	if idx >= 250 {
		newIndex := 0
		var newConnectionList = [256]connection{}
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
