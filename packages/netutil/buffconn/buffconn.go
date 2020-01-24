package buffconn

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
)

const (
	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize = 4096
	// IOTimeout specifies the timeout for sending and receiving multi packet messages.
	IOTimeout = 4 * time.Second
)

// Errors returned by the BufferedConnection.
var (
	ErrInvalidHeader      = errors.New("invalid message header")
	ErrInsufficientBuffer = errors.New("insufficient buffer")
)

// BufferedConnectionEvents contains all the events that are triggered during the peer discovery.
type BufferedConnectionEvents struct {
	ReceiveMessage *events.Event
	Close          *events.Event
}

// BufferedConnection is a wrapper for sending and reading messages with a buffer.
type BufferedConnection struct {
	Events BufferedConnectionEvents

	conn                 net.Conn
	headerByteSize       int
	incomingHeaderBuffer []byte
	closeOnce            sync.Once
}

// NewBufferedConnection creates a new BufferedConnection from a net.Conn.
func NewBufferedConnection(conn net.Conn) *BufferedConnection {
	// compute the maximum number of bytes to encode the message size
	lenByteSize := binary.PutVarint(make([]byte, binary.MaxVarintLen64), MaxMessageSize)
	return &BufferedConnection{
		conn: conn,
		Events: BufferedConnectionEvents{
			ReceiveMessage: events.NewEvent(events.ByteSliceCaller),
			Close:          events.NewEvent(events.CallbackCaller),
		},
		headerByteSize:       lenByteSize,
		incomingHeaderBuffer: make([]byte, lenByteSize),
	}
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *BufferedConnection) Close() (err error) {
	c.closeOnce.Do(func() {
		err = c.conn.Close()
		// close in separate go routine to avoid deadlocks
		go c.Events.Close.Trigger()
	})
	return err
}

// LocalAddr returns the local network address.
func (c *BufferedConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *BufferedConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Write sends a stream of bytes as messages.
// Each array of bytes you pass in will be pre-pended with it's size. If the
// connection isn't open you will receive an error. If not all bytes can be
// written, Write will keep trying until the full message is delivered, or the
// connection is broken.
func (c *BufferedConnection) Write(data []byte) (int, error) {
	if len(data) > MaxMessageSize {
		panic(nil)
	}

	header := make([]byte, c.headerByteSize)
	binary.PutVarint(header, int64(len(data)))

	buffer := append(header, data...)

	if err := c.conn.SetWriteDeadline(time.Now().Add(IOTimeout)); err != nil {
		return 0, fmt.Errorf("error while setting timeout: %w", err)
	}

	toWrite := len(buffer)
	for bytesWritten := 0; bytesWritten < toWrite; {
		n, err := c.conn.Write(buffer[bytesWritten:])
		bytesWritten += n
		if err != nil {
			return bytesWritten, err
		}
	}
	return toWrite - c.headerByteSize, nil
}

func (c *BufferedConnection) read(buffer []byte) (int, error) {
	toRead := len(buffer)
	for bytesRead := 0; bytesRead < toRead; {
		n, err := c.conn.Read(buffer[bytesRead:])
		bytesRead += n
		if err != nil {
			return bytesRead, err
		}
	}
	return toRead, nil
}

func (c *BufferedConnection) readMessage(buffer []byte) (int, error) {
	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return 0, fmt.Errorf("error while unsetting timeout: %w", err)
	}
	_, err := c.read(c.incomingHeaderBuffer)
	if err != nil {
		return 0, err
	}

	msgLength, bytesRead := binary.Varint(c.incomingHeaderBuffer)
	if bytesRead <= 0 || msgLength > MaxMessageSize {
		return 0, ErrInvalidHeader
	}
	if msgLength > int64(len(buffer)) {
		return 0, ErrInsufficientBuffer
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(IOTimeout)); err != nil {
		return 0, fmt.Errorf("error while setting timeout: %w", err)
	}
	return c.read(buffer[:msgLength])
}

// Read starts reading on the connection, it only returns when an error occurred or when Close has been called.
// If a complete message has been received and ReceiveMessage event is triggered with its complete payload.
// If read leads to an error, the loop will be stopped and that error returned.
func (c *BufferedConnection) Read() error {
	buffer := make([]byte, MaxMessageSize)

	for {
		n, err := c.readMessage(buffer)
		if err != nil {
			return err
		}
		if n > 0 {
			c.Events.ReceiveMessage.Trigger(buffer[:n])
		}
	}
}
